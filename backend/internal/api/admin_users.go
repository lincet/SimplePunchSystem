package api

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"simplepunch/internal/auth"
	"simplepunch/internal/store"
)

type userInput struct {
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Gender     string `json:"gender"`
	EmployeeNo string `json:"employeeNo"`
	Title      string `json:"title"`
	HireDate   string `json:"hireDate"`
	Email      string `json:"email"`
	Note       string `json:"note"`
	Active     *bool  `json:"active"`
	Password   string `json:"password"` // 只在新增時使用
}

var (
	phoneRe = regexp.MustCompile(`^\+?[0-9]{6,15}$`)
	emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// validate 清理並檢查欄位，回傳錯誤訊息（空字串表示通過）。
func (in *userInput) validate() string {
	in.Name = strings.TrimSpace(in.Name)
	in.Phone = NormalizeUsername(in.Phone)
	in.EmployeeNo = strings.TrimSpace(in.EmployeeNo)
	in.Title = strings.TrimSpace(in.Title)
	in.HireDate = strings.TrimSpace(in.HireDate)
	in.Email = strings.TrimSpace(in.Email)
	in.Note = strings.TrimSpace(in.Note)
	switch {
	case in.Name == "":
		return "請輸入姓名"
	case utf8.RuneCountInString(in.Name) > 50:
		return "姓名太長"
	case !phoneRe.MatchString(in.Phone):
		return "電話格式錯誤（只能是數字，6~15 碼）"
	case in.Gender != "male" && in.Gender != "female" && in.Gender != "other":
		return "請選擇性別"
	case in.Email != "" && !emailRe.MatchString(in.Email):
		return "Email 格式錯誤"
	case utf8.RuneCountInString(in.Note) > 500:
		return "備註太長"
	}
	if in.HireDate != "" {
		if _, err := time.Parse(time.DateOnly, in.HireDate); err != nil {
			return "到職日格式錯誤"
		}
	}
	return ""
}

func (in *userInput) apply(u *store.User) {
	u.Name, u.Phone, u.Username = in.Name, in.Phone, in.Phone // 員工以電話號碼登入
	u.Gender, u.EmployeeNo, u.Title = in.Gender, in.EmployeeNo, in.Title
	u.HireDate, u.Email, u.Note = in.HireDate, in.Email, in.Note
	if in.Active != nil {
		u.Active = *in.Active
	}
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.st.ListEmployees(r.URL.Query().Get("all") == "1")
	if err != nil {
		serverError(w, err)
		return
	}
	if users == nil {
		users = []*store.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) loadEmployee(w http.ResponseWriter, r *http.Request) *store.User {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err == nil {
		u, err := s.st.GetUser(id)
		if err == nil && u.Role == "employee" {
			return u
		}
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			serverError(w, err)
			return nil
		}
	}
	writeError(w, http.StatusNotFound, "not_found", "找不到此員工")
	return nil
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	if u := s.loadEmployee(w, r); u != nil {
		writeJSON(w, http.StatusOK, u)
	}
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var in userInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, "invalid", msg)
		return
	}
	if err := auth.ValidatePassword(in.Password); err != nil {
		writeError(w, http.StatusBadRequest, "weak_password", "預設密碼至少需要 8 個字元")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		serverError(w, err)
		return
	}
	u := &store.User{Role: "employee", Active: true, MustChangePassword: true, PasswordHash: hash}
	in.apply(u)
	if err := s.st.CreateUser(u); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			writeError(w, http.StatusConflict, "duplicate", "這個電話號碼已經有人使用")
			return
		}
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	u := s.loadEmployee(w, r)
	if u == nil {
		return
	}
	var in userInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := in.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, "invalid", msg)
		return
	}
	in.apply(u)
	if err := s.st.UpdateProfile(u); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			writeError(w, http.StatusConflict, "duplicate", "這個電話號碼已經有人使用")
			return
		}
		serverError(w, err)
		return
	}
	if !u.Active {
		s.st.DeleteUserSessions(u.ID, "")
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	u := s.loadEmployee(w, r)
	if u == nil {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, "weak_password", "預設密碼至少需要 8 個字元")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		serverError(w, err)
		return
	}
	if err := s.st.SetPassword(u.ID, hash, true); err != nil {
		serverError(w, err)
		return
	}
	s.st.DeleteUserSessions(u.ID, "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
