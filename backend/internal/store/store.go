// Package store 封裝所有 SQL 存取。時間一律以固定寬度的 UTC 字串存入，可直接字串比較排序。
package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

const timeLayout = "2006-01-02T15:04:05.000000Z"

var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("duplicate")
)

func FormatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func ParseTime(s string) time.Time {
	t, _ := time.Parse(timeLayout, s)
	return t
}

type Store struct{ DB *sql.DB }

func New(db *sql.DB) *Store { return &Store{DB: db} }

// ---------- Users ----------

type User struct {
	ID                 int64  `json:"id"`
	Role               string `json:"role"`
	Username           string `json:"username"`
	Name               string `json:"name"`
	Phone              string `json:"phone"`
	Gender             string `json:"gender"`
	EmployeeNo         string `json:"employeeNo"`
	Title              string `json:"title"`
	HireDate           string `json:"hireDate"`
	Email              string `json:"email"`
	Note               string `json:"note"`
	PasswordHash       string `json:"-"`
	MustChangePassword bool   `json:"mustChangePassword"`
	Active             bool   `json:"active"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

const userCols = `id, role, username, name, phone, gender, employee_no, title, hire_date, email, note,
	password_hash, must_change_password, active, created_at, updated_at`

type scanner interface{ Scan(dest ...any) error }

func scanUser(s scanner) (*User, error) {
	var u User
	err := s.Scan(&u.ID, &u.Role, &u.Username, &u.Name, &u.Phone, &u.Gender, &u.EmployeeNo, &u.Title,
		&u.HireDate, &u.Email, &u.Note, &u.PasswordHash, &u.MustChangePassword, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func (s *Store) CreateUser(u *User) error {
	now := FormatTime(time.Now())
	u.CreatedAt, u.UpdatedAt = now, now
	res, err := s.DB.Exec(`INSERT INTO users (role, username, name, phone, gender, employee_no, title, hire_date,
		email, note, password_hash, must_change_password, active, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		u.Role, u.Username, u.Name, u.Phone, u.Gender, u.EmployeeNo, u.Title, u.HireDate, u.Email, u.Note,
		u.PasswordHash, u.MustChangePassword, u.Active, u.CreatedAt, u.UpdatedAt)
	if isUnique(err) {
		return ErrDuplicate
	}
	if err != nil {
		return err
	}
	u.ID, _ = res.LastInsertId()
	return nil
}

// UpdateProfile 更新老闆可編輯的欄位（不含密碼）。
func (s *Store) UpdateProfile(u *User) error {
	u.UpdatedAt = FormatTime(time.Now())
	_, err := s.DB.Exec(`UPDATE users SET username=?, name=?, phone=?, gender=?, employee_no=?, title=?,
		hire_date=?, email=?, note=?, active=?, updated_at=? WHERE id=?`,
		u.Username, u.Name, u.Phone, u.Gender, u.EmployeeNo, u.Title, u.HireDate, u.Email, u.Note, u.Active,
		u.UpdatedAt, u.ID)
	if isUnique(err) {
		return ErrDuplicate
	}
	return err
}

func (s *Store) SetPassword(userID int64, hash string, mustChange bool) error {
	_, err := s.DB.Exec(`UPDATE users SET password_hash=?, must_change_password=?, updated_at=? WHERE id=?`,
		hash, mustChange, FormatTime(time.Now()), userID)
	return err
}

func (s *Store) GetUser(id int64) (*User, error) {
	return scanUser(s.DB.QueryRow(`SELECT `+userCols+` FROM users WHERE id=?`, id))
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	return scanUser(s.DB.QueryRow(`SELECT `+userCols+` FROM users WHERE username=?`, username))
}

func (s *Store) ListEmployees(includeInactive bool) ([]*User, error) {
	q := `SELECT ` + userCols + ` FROM users WHERE role='employee'`
	if !includeInactive {
		q += ` AND active=1`
	}
	q += ` ORDER BY active DESC, employee_no, id`
	rows, err := s.DB.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CountAdmins() (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role='admin'`).Scan(&n)
	return n, err
}

// ---------- Sessions ----------

func (s *Store) CreateSession(tokenHash string, userID int64, expires time.Time) error {
	_, err := s.DB.Exec(`INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?,?,?,?)`,
		tokenHash, userID, FormatTime(expires), FormatTime(time.Now()))
	return err
}

// UserBySession 回傳有效 session 對應的啟用中使用者。
func (s *Store) UserBySession(tokenHash string) (*User, error) {
	return scanUser(s.DB.QueryRow(`SELECT `+prefixCols("u.")+` FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash=? AND s.expires_at > ? AND u.active=1`, tokenHash, FormatTime(time.Now())))
}

func prefixCols(p string) string {
	parts := strings.Split(userCols, ",")
	for i, c := range parts {
		parts[i] = p + strings.TrimSpace(c)
	}
	return strings.Join(parts, ", ")
}

func (s *Store) DeleteSession(tokenHash string) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE token_hash=?`, tokenHash)
	return err
}

// DeleteUserSessions 刪除使用者所有 session；keep 非空時保留該 session（改密碼後保留目前裝置的登入）。
func (s *Store) DeleteUserSessions(userID int64, keep string) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE user_id=? AND token_hash<>?`, userID, keep)
	return err
}

func (s *Store) PurgeExpiredSessions() error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, FormatTime(time.Now()))
	return err
}

// ---------- Punches ----------

type Punch struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"userId"`
	UserName  string    `json:"userName,omitempty"`
	Type      string    `json:"type"`
	PunchedAt time.Time `json:"punchedAt"`
	PhotoPath string    `json:"-"`
	UserAgent string    `json:"-"`
	IP        string    `json:"-"`
}

func (s *Store) CreatePunch(p *Punch) error {
	res, err := s.DB.Exec(`INSERT INTO punches (user_id, type, punched_at, photo_path, user_agent, ip, created_at)
		VALUES (?,?,?,?,?,?,?)`, p.UserID, p.Type, FormatTime(p.PunchedAt), p.PhotoPath, p.UserAgent, p.IP,
		FormatTime(time.Now()))
	if err != nil {
		return err
	}
	p.ID, _ = res.LastInsertId()
	return nil
}

func (s *Store) LastPunch(userID int64) (*Punch, error) {
	row := s.DB.QueryRow(`SELECT p.id, p.user_id, u.name, p.type, p.punched_at, p.photo_path FROM punches p
		JOIN users u ON u.id = p.user_id WHERE p.user_id=? ORDER BY p.punched_at DESC, p.id DESC LIMIT 1`, userID)
	p, err := scanPunch(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func (s *Store) GetPunch(id int64) (*Punch, error) {
	row := s.DB.QueryRow(`SELECT p.id, p.user_id, u.name, p.type, p.punched_at, p.photo_path FROM punches p
		JOIN users u ON u.id = p.user_id WHERE p.id=?`, id)
	p, err := scanPunch(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func scanPunch(s scanner) (*Punch, error) {
	var p Punch
	var at string
	if err := s.Scan(&p.ID, &p.UserID, &p.UserName, &p.Type, &at, &p.PhotoPath); err != nil {
		return nil, err
	}
	p.PunchedAt = ParseTime(at)
	return &p, nil
}

// ListPunches 列出 [from, to) 區間內的打卡；userID 為 0 表示全部員工。依時間由舊到新。
func (s *Store) ListPunches(userID int64, from, to time.Time) ([]*Punch, error) {
	q := `SELECT p.id, p.user_id, u.name, p.type, p.punched_at, p.photo_path FROM punches p
		JOIN users u ON u.id = p.user_id WHERE p.punched_at >= ? AND p.punched_at < ?`
	args := []any{FormatTime(from), FormatTime(to)}
	if userID != 0 {
		q += ` AND p.user_id = ?`
		args = append(args, userID)
	}
	q += ` ORDER BY p.punched_at, p.id`
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Punch
	for rows.Next() {
		p, err := scanPunch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
