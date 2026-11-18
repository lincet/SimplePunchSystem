package api

import (
	"net/http"
	"strconv"
	"time"

	"simplepunch/internal/hours"
	"simplepunch/internal/store"
)

type userRef struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	EmployeeNo string `json:"employeeNo"`
	Title      string `json:"title"`
	Active     bool   `json:"active"`
}

type UserReport struct {
	User    userRef       `json:"user"`
	Summary hours.Summary `json:"summary"`
}

func toHourPunches(ps []*store.Punch) []hours.Punch {
	out := make([]hours.Punch, len(ps))
	for i, p := range ps {
		out[i] = hours.Punch{ID: p.ID, Type: p.Type, At: p.PunchedAt}
	}
	return out
}

// punchWindowEnd 查詢打卡時多抓一段時間，讓區間最後一天的跨夜班能配到下班卡。
func punchWindowEnd(end time.Time) time.Time { return end.Add(hours.MaxShift) }

func (s *Server) userSegments(userID int64, start, end time.Time) ([]hours.Segment, error) {
	ps, err := s.st.ListPunches(userID, start, punchWindowEnd(end))
	if err != nil {
		return nil, err
	}
	segs := hours.Pair(toHourPunches(ps), s.cfg.Loc)
	hours.MarkOpen(segs, s.now())
	return segs, nil
}

// buildReports 產生每位員工在 [start, end) 的出勤彙總。userID 為 0 表示全部員工；
// 已停用且區間內沒有出勤的員工會被略過。
func (s *Server) buildReports(userID int64, start, end time.Time) ([]UserReport, error) {
	var users []*store.User
	if userID != 0 {
		u, err := s.st.GetUser(userID)
		if err != nil {
			return nil, err
		}
		users = []*store.User{u}
	} else {
		var err error
		if users, err = s.st.ListEmployees(true); err != nil {
			return nil, err
		}
	}
	ps, err := s.st.ListPunches(userID, start, punchWindowEnd(end))
	if err != nil {
		return nil, err
	}
	byUser := map[int64][]*store.Punch{}
	for _, p := range ps {
		byUser[p.UserID] = append(byUser[p.UserID], p)
	}
	reports := []UserReport{}
	for _, u := range users {
		segs := hours.Pair(toHourPunches(byUser[u.ID]), s.cfg.Loc)
		hours.MarkOpen(segs, s.now())
		sum := hours.Summarize(segs, start, end, s.cfg.Loc)
		if !u.Active && sum.DaysWorked == 0 && userID == 0 {
			continue
		}
		reports = append(reports, UserReport{
			User:    userRef{ID: u.ID, Name: u.Name, EmployeeNo: u.EmployeeNo, Title: u.Title, Active: u.Active},
			Summary: sum,
		})
	}
	return reports, nil
}

func queryUserID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	return id
}

func (s *Server) handleAttendance(w http.ResponseWriter, r *http.Request) {
	start, end, err := s.parseDateRange(r, 7)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_range", err.Error())
		return
	}
	reports, err := s.buildReports(queryUserID(r), start, end)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from":    start.Format(time.DateOnly),
		"to":      end.AddDate(0, 0, -1).Format(time.DateOnly),
		"reports": reports,
	})
}

func (s *Server) handleHours(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period != "day" && period != "week" && period != "month" {
		period = "day"
	}
	date := s.now()
	if v := r.URL.Query().Get("date"); v != "" {
		t, err := time.ParseInLocation(time.DateOnly, v, s.cfg.Loc)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_date", "日期格式錯誤")
			return
		}
		date = t
	}
	start, end := hours.Range(period, date, s.cfg.Loc)
	reports, err := s.buildReports(queryUserID(r), start, end)
	if err != nil {
		serverError(w, err)
		return
	}
	var total float64
	for _, rep := range reports {
		total += rep.Summary.TotalHours
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"period":     period,
		"from":       start.Format(time.DateOnly),
		"to":         end.AddDate(0, 0, -1).Format(time.DateOnly),
		"totalHours": hours.Round2(total),
		"reports":    reports,
	})
}
