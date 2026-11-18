package api

import (
	"errors"
	"net/http"
	"strings"

	"simplepunch/internal/auth"
	"simplepunch/internal/store"
)

// NormalizeUsername 去掉空白與連字號，讓「0912-345-678」與「0912345678」視為同一帳號。
func NormalizeUsername(s string) string {
	return strings.NewReplacer(" ", "", "-", "", "　", "").Replace(strings.TrimSpace(s))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{"ok": true, "demo": s.cfg.DemoSeed, "timezone": s.cfg.TZName}
	if s.cfg.DemoSeed {
		resp["demoAccounts"] = s.DemoAccounts
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	username := NormalizeUsername(req.Username)
	ip := clientIP(r)
	userKey, ipKey := "u:"+username, "ip:"+ip
	if s.userLimiter.Blocked(userKey) || s.ipLimiter.Blocked(ipKey) {
		writeError(w, http.StatusTooManyRequests, "too_many_attempts", "登入失敗次數過多，請 15 分鐘後再試")
		return
	}
	u, err := s.st.GetUserByUsername(username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		serverError(w, err)
		return
	}
	if u == nil || !auth.CheckPassword(u.PasswordHash, req.Password) {
		s.userLimiter.Fail(userKey)
		s.ipLimiter.Fail(ipKey)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "帳號或密碼錯誤")
		return
	}
	if !u.Active {
		writeError(w, http.StatusForbidden, "inactive", "此帳號已停用，請聯絡管理者")
		return
	}
	s.userLimiter.Reset(userKey)
	tok, hash, err := auth.NewToken()
	if err != nil {
		serverError(w, err)
		return
	}
	expires := s.now().Add(auth.SessionDuration)
	if err := s.st.CreateSession(hash, u.ID, expires); err != nil {
		serverError(w, err)
		return
	}
	s.setCookie(w, tok, expires)
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.st.DeleteSession(currentToken(r))
	s.clearCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r))
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	u := currentUser(r)
	if !auth.CheckPassword(u.PasswordHash, req.OldPassword) {
		writeError(w, http.StatusBadRequest, "wrong_password", "目前密碼不正確")
		return
	}
	if err := auth.ValidatePassword(req.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, "weak_password", err.Error())
		return
	}
	if req.NewPassword == req.OldPassword {
		writeError(w, http.StatusBadRequest, "same_password", "新密碼不可與目前密碼相同")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		serverError(w, err)
		return
	}
	if err := s.st.SetPassword(u.ID, hash, false); err != nil {
		serverError(w, err)
		return
	}
	// 登出其他裝置，保留目前這支手機的登入。
	s.st.DeleteUserSessions(u.ID, currentToken(r))
	u.MustChangePassword = false
	writeJSON(w, http.StatusOK, u)
}
