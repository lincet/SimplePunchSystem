// Package api 實作 HTTP API。
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"simplepunch/internal/auth"
	"simplepunch/internal/config"
	"simplepunch/internal/photos"
	"simplepunch/internal/store"
)

type DemoAccount struct {
	Label    string `json:"label"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Server struct {
	cfg          *config.Config
	st           *store.Store
	photos       *photos.Store
	userLimiter  *auth.Limiter
	ipLimiter    *auth.Limiter
	punchMu      sync.Mutex
	now          func() time.Time
	DemoAccounts []DemoAccount
}

func New(cfg *config.Config, st *store.Store, ph *photos.Store) *Server {
	return &Server{
		cfg:         cfg,
		st:          st,
		photos:      ph,
		userLimiter: auth.NewLimiter(5, 15*time.Minute),
		ipLimiter:   auth.NewLimiter(30, 15*time.Minute),
		now:         time.Now,
	}
}

// access 描述 endpoint 的權限需求。
type access int

const (
	public       access = iota
	anyUser             // 已登入（即使尚未更改預設密碼）
	employeeOnly        // 已登入員工，且已改密碼
	signedIn            // 已登入任一角色，且已改密碼
	adminOnly           // 已登入老闆，且已改密碼
)

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	route := func(pattern string, a access, h http.HandlerFunc) {
		mux.Handle(pattern, s.guard(a, h))
	}

	route("GET /api/health", public, s.handleHealth)
	route("POST /api/auth/login", public, s.handleLogin)
	route("POST /api/auth/logout", anyUser, s.handleLogout)
	route("GET /api/me", anyUser, s.handleMe)
	route("POST /api/me/password", anyUser, s.handleChangePassword)

	route("GET /api/punch/status", employeeOnly, s.handlePunchStatus)
	route("POST /api/punch", employeeOnly, s.handlePunch)
	route("GET /api/me/punches", employeeOnly, s.handleMyPunches)
	route("GET /api/photos/{id}", signedIn, s.handlePhoto)

	route("GET /api/admin/users", adminOnly, s.handleListUsers)
	route("POST /api/admin/users", adminOnly, s.handleCreateUser)
	route("GET /api/admin/users/{id}", adminOnly, s.handleGetUser)
	route("PUT /api/admin/users/{id}", adminOnly, s.handleUpdateUser)
	route("POST /api/admin/users/{id}/reset-password", adminOnly, s.handleResetPassword)
	route("GET /api/admin/attendance", adminOnly, s.handleAttendance)
	route("GET /api/admin/hours", adminOnly, s.handleHours)
	route("GET /api/admin/export", adminOnly, s.handleExport)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "找不到此 API")
	})
	if s.cfg.StaticDir != "" {
		mux.Handle("/", spaHandler(s.cfg.StaticDir))
	}
	return logRequests(securityHeaders(mux))
}

// ---------- middleware ----------

type ctxKey int

const (
	userKey ctxKey = iota
	tokenKey
)

func currentUser(r *http.Request) *store.User {
	u, _ := r.Context().Value(userKey).(*store.User)
	return u
}

func currentToken(r *http.Request) string {
	t, _ := r.Context().Value(tokenKey).(string)
	return t
}

func (s *Server) guard(a access, h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 簡單的 CSRF 防護：所有會改變狀態的請求都必須帶自訂標頭（跨站表單無法帶）。
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("X-Requested-With") != "fetch" {
			writeError(w, http.StatusForbidden, "csrf", "請求來源不正確")
			return
		}
		if a == public {
			h(w, r)
			return
		}
		c, err := r.Cookie(auth.SessionCookie)
		if err != nil || c.Value == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "請先登入")
			return
		}
		hash := auth.HashToken(c.Value)
		u, err := s.st.UserBySession(hash)
		if err != nil {
			s.clearCookie(w)
			writeError(w, http.StatusUnauthorized, "unauthorized", "登入已過期，請重新登入")
			return
		}
		if a != anyUser && u.MustChangePassword {
			writeError(w, http.StatusForbidden, "password_change_required", "請先更改預設密碼")
			return
		}
		if a == adminOnly && u.Role != "admin" {
			writeError(w, http.StatusForbidden, "forbidden", "只有管理者可以使用此功能")
			return
		}
		if a == employeeOnly && u.Role != "employee" {
			writeError(w, http.StatusForbidden, "forbidden", "只有員工可以使用此功能")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, u)
		ctx = context.WithValue(ctx, tokenKey, hash)
		h(w, r.WithContext(ctx))
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/api/photos/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
		}
	})
}

// spaHandler 提供前端靜態檔案（本機開發用），找不到的路徑回 index.html。
func spaHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if st, err := os.Stat(p); err != nil || st.IsDir() && r.URL.Path != "/" {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		if strings.HasSuffix(r.URL.Path, "sw.js") || r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cache-Control", "no-cache")
		}
		fs.ServeHTTP(w, r)
	})
}

// ---------- helpers ----------

type apiError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiError{Error: code, Message: msg})
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal", "伺服器發生錯誤，請稍後再試")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "資料格式錯誤")
		return false
	}
	return true
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Nf-Client-Connection-Ip"); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) setCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: auth.SessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

// parseDateRange 解析 from/to（YYYY-MM-DD，皆含），回傳 [start, end)。
func (s *Server) parseDateRange(r *http.Request, defaultDays int) (time.Time, time.Time, error) {
	loc := s.cfg.Loc
	today := time.Date(s.now().In(loc).Year(), s.now().In(loc).Month(), s.now().In(loc).Day(), 0, 0, 0, 0, loc)
	end := today.AddDate(0, 0, 1)
	start := end.AddDate(0, 0, -defaultDays)
	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.ParseInLocation(time.DateOnly, v, loc)
		if err != nil {
			return start, end, errors.New("from 日期格式錯誤")
		}
		start = t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.ParseInLocation(time.DateOnly, v, loc)
		if err != nil {
			return start, end, errors.New("to 日期格式錯誤")
		}
		end = t.AddDate(0, 0, 1)
	}
	if !start.Before(end) {
		return start, end, errors.New("開始日期不可晚於結束日期")
	}
	if end.Sub(start) > 400*24*time.Hour {
		return start, end, errors.New("查詢區間最多一年")
	}
	return start, end, nil
}
