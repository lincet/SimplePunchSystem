package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"simplepunch/internal/hours"
	"simplepunch/internal/photos"
	"simplepunch/internal/store"
)

type punchJSON struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	PunchedAt time.Time `json:"punchedAt"`
	PhotoURL  string    `json:"photoUrl"`
}

func toPunchJSON(p *store.Punch) punchJSON {
	return punchJSON{ID: p.ID, Type: p.Type, PunchedAt: p.PunchedAt, PhotoURL: photoURL(p.ID)}
}

func photoURL(punchID int64) string {
	if punchID == 0 {
		return ""
	}
	return fmt.Sprintf("/api/photos/%d", punchID)
}

func (s *Server) punchStatus(userID int64) (map[string]any, error) {
	now := s.now()
	last, err := s.st.LastPunch(userID)
	if err != nil {
		return nil, err
	}
	next := "in"
	var lastJSON any
	if last != nil {
		next = hours.NextType(last.Type, last.PunchedAt, now)
		lastJSON = toPunchJSON(last)
	}
	start, end := hours.Range("day", now, s.cfg.Loc)
	today, err := s.st.ListPunches(userID, start, end)
	if err != nil {
		return nil, err
	}
	todayJSON := make([]punchJSON, 0, len(today))
	for _, p := range today {
		todayJSON = append(todayJSON, toPunchJSON(p))
	}
	return map[string]any{
		"next":       next,
		"last":       lastJSON,
		"today":      todayJSON,
		"serverTime": now,
	}, nil
}

func (s *Server) handlePunchStatus(w http.ResponseWriter, r *http.Request) {
	st, err := s.punchStatus(currentUser(r).ID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handlePunch(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	r.Body = http.MaxBytesReader(w, r.Body, photos.MaxSize+(512<<10))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "上傳資料錯誤或照片太大")
		return
	}
	typ := r.FormValue("type")
	if typ != "in" && typ != "out" {
		writeError(w, http.StatusBadRequest, "bad_type", "打卡類型錯誤")
		return
	}
	file, _, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "photo_required", "打卡必須附上照片")
		return
	}
	defer file.Close()

	s.punchMu.Lock()
	defer s.punchMu.Unlock()

	now := s.now()
	last, err := s.st.LastPunch(u.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	expected := "in"
	if last != nil {
		expected = hours.NextType(last.Type, last.PunchedAt, now)
	}
	if typ != expected {
		msg := "你已經打過上班卡了，下一次請打下班卡"
		if expected == "in" {
			msg = "你目前不在上班狀態，請先打上班卡"
		}
		writeError(w, http.StatusConflict, "wrong_state", msg)
		return
	}
	rel, err := s.photos.Save(file, now)
	if err != nil {
		if errors.Is(err, photos.ErrTooLarge) || errors.Is(err, photos.ErrNotAnImage) || errors.Is(err, photos.ErrEmpty) {
			writeError(w, http.StatusBadRequest, "bad_photo", err.Error())
			return
		}
		serverError(w, err)
		return
	}
	p := &store.Punch{UserID: u.ID, Type: typ, PunchedAt: now, PhotoPath: rel, UserAgent: r.UserAgent(), IP: clientIP(r)}
	if err := s.st.CreatePunch(p); err != nil {
		serverError(w, err)
		return
	}
	st, err := s.punchStatus(u.ID)
	if err != nil {
		serverError(w, err)
		return
	}
	st["punch"] = toPunchJSON(p)
	writeJSON(w, http.StatusCreated, st)
}

func (s *Server) handleMyPunches(w http.ResponseWriter, r *http.Request) {
	start, end, err := s.parseDateRange(r, 14)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_range", err.Error())
		return
	}
	u := currentUser(r)
	segs, err := s.userSegments(u.ID, start, end)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, hours.Summarize(segs, start, end, s.cfg.Loc))
}

func (s *Server) handlePhoto(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "找不到照片")
		return
	}
	p, err := s.st.GetPunch(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "找不到照片")
		return
	}
	u := currentUser(r)
	if u.Role != "admin" && p.UserID != u.ID {
		writeError(w, http.StatusForbidden, "forbidden", "無權查看此照片")
		return
	}
	path, err := s.photos.Path(p.PhotoPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "找不到照片")
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, path)
}
