// Package hours 負責把打卡紀錄配對成上班時段，並計算單日／單週／單月工時。
//
// 規則：
//   - 依時間排序後，每個「上班」與其後第一個「下班」配對成一段。
//   - 時段歸屬於「上班打卡」的日期（公司時區），因此跨夜班算在上班當天。
//   - 上班後沒有下班卡（或下一筆又是上班）→ 該段標記為未完成，不計入時數。
//   - 沒有對應上班卡的下班卡會被忽略（通常屬於查詢區間之前的班）。
package hours

import (
	"math"
	"sort"
	"time"
)

// MaxShift 上班超過這個時間仍未下班，視為忘記打下班卡，允許重新上班打卡。
const MaxShift = 20 * time.Hour

type Punch struct {
	ID   int64
	Type string // "in" | "out"
	At   time.Time
}

type Segment struct {
	Date     string     `json:"date"` // YYYY-MM-DD（上班日，公司時區）
	In       time.Time  `json:"in"`
	Out      *time.Time `json:"out"`
	InID     int64      `json:"inId"`
	OutID    int64      `json:"outId,omitempty"`
	Minutes  float64    `json:"minutes"`
	Complete bool       `json:"complete"`
	Open     bool       `json:"open"` // 尚未下班但仍在合理上班時間內（上班中），不算忘記打卡
}

// MarkOpen 把「上班後還沒超過 MaxShift」的未完成時段標記為上班中。
func MarkOpen(segs []Segment, now time.Time) {
	for i := range segs {
		if !segs[i].Complete && now.Sub(segs[i].In) < MaxShift {
			segs[i].Open = true
		}
	}
}

// Pair 將打卡配對成時段。
func Pair(punches []Punch, loc *time.Location) []Segment {
	ps := append([]Punch(nil), punches...)
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].At.Before(ps[j].At) })

	var segs []Segment
	var open *Punch
	flush := func() {
		if open != nil {
			segs = append(segs, Segment{Date: open.At.In(loc).Format(time.DateOnly), In: open.At, InID: open.ID})
			open = nil
		}
	}
	for i := range ps {
		p := ps[i]
		switch p.Type {
		case "in":
			flush()
			open = &p
		case "out":
			if open == nil {
				continue
			}
			out := p.At
			segs = append(segs, Segment{
				Date:     open.At.In(loc).Format(time.DateOnly),
				In:       open.At,
				Out:      &out,
				InID:     open.ID,
				OutID:    p.ID,
				Minutes:  out.Sub(open.At).Minutes(),
				Complete: true,
			})
			open = nil
		}
	}
	flush()
	return segs
}

type Day struct {
	Date       string    `json:"date"`
	Hours      float64   `json:"hours"`
	Segments   []Segment `json:"segments"`
	Incomplete int       `json:"incomplete"` // 未打下班卡的段數（不含上班中）
}

type Summary struct {
	From       string  `json:"from"` // 含
	To         string  `json:"to"`   // 含
	TotalHours float64 `json:"totalHours"`
	DaysWorked int     `json:"daysWorked"`
	Incomplete int     `json:"incomplete"`
	Days       []Day   `json:"days"`
}

// Summarize 將時段依日期彙總；只保留上班日落在 [from, to) 的時段，且區間內每一天都會出現（沒上班為 0）。
func Summarize(segs []Segment, from, to time.Time, loc *time.Location) Summary {
	byDate := map[string]*Day{}
	var order []string
	for d := from.In(loc); d.Before(to); d = d.AddDate(0, 0, 1) {
		key := d.Format(time.DateOnly)
		byDate[key] = &Day{Date: key, Segments: []Segment{}}
		order = append(order, key)
	}
	sum := Summary{From: from.In(loc).Format(time.DateOnly), To: to.In(loc).AddDate(0, 0, -1).Format(time.DateOnly)}
	for _, s := range segs {
		day, ok := byDate[s.Date]
		if !ok {
			continue
		}
		day.Segments = append(day.Segments, s)
		if s.Complete {
			day.Hours += s.Minutes / 60
		} else if !s.Open {
			day.Incomplete++
		}
	}
	for _, key := range order {
		d := byDate[key]
		d.Hours = Round2(d.Hours)
		sum.TotalHours += d.Hours
		if len(d.Segments) > 0 {
			sum.DaysWorked++
		}
		sum.Incomplete += d.Incomplete
		sum.Days = append(sum.Days, *d)
	}
	sum.TotalHours = Round2(sum.TotalHours)
	return sum
}

func Round2(v float64) float64 { return math.Round(v*100) / 100 }

// StartOfDay 回傳 t 在 loc 時區當天 00:00。
func StartOfDay(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// Range 回傳 period（day/week/month）包含 date 的區間 [start, end)。週為週一到週日。
func Range(period string, date time.Time, loc *time.Location) (time.Time, time.Time) {
	d := StartOfDay(date, loc)
	switch period {
	case "week":
		offset := (int(d.Weekday()) + 6) % 7 // 週一 = 0
		start := d.AddDate(0, 0, -offset)
		return start, start.AddDate(0, 0, 7)
	case "month":
		start := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, loc)
		return start, start.AddDate(0, 1, 0)
	default:
		return d, d.AddDate(0, 0, 1)
	}
}

// NextType 根據最後一筆打卡決定下一次應該打上班還是下班。
func NextType(lastType string, lastAt time.Time, now time.Time) string {
	if lastType == "in" && now.Sub(lastAt) < MaxShift {
		return "out"
	}
	return "in"
}
