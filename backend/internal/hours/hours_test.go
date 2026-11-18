package hours

import (
	"testing"
	"time"
)

var tpe = mustLoc("Asia/Taipei")

func mustLoc(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return l
}

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, tpe)
	if err != nil {
		panic(err)
	}
	return t
}

func p(typ, s string) Punch { return Punch{Type: typ, At: at(s)} }

func TestNormalDay(t *testing.T) {
	segs := Pair([]Punch{p("in", "2026-11-16 09:00"), p("out", "2026-11-16 18:30")}, tpe)
	if len(segs) != 1 || !segs[0].Complete || segs[0].Minutes != 570 {
		t.Fatalf("unexpected %+v", segs)
	}
	s := Summarize(segs, at("2026-11-16 00:00"), at("2026-11-17 00:00"), tpe)
	if s.TotalHours != 9.5 || s.DaysWorked != 1 || len(s.Days) != 1 {
		t.Fatalf("unexpected %+v", s)
	}
}

func TestMultipleSegmentsAndUnsorted(t *testing.T) {
	segs := Pair([]Punch{
		p("out", "2026-11-16 18:00"), p("in", "2026-11-16 13:00"),
		p("out", "2026-11-16 12:00"), p("in", "2026-11-16 09:00"),
	}, tpe)
	s := Summarize(segs, at("2026-11-16 00:00"), at("2026-11-17 00:00"), tpe)
	if len(segs) != 2 || s.TotalHours != 8 {
		t.Fatalf("segs=%+v sum=%+v", segs, s)
	}
}

func TestOvernightShiftCountsOnStartDay(t *testing.T) {
	segs := Pair([]Punch{p("in", "2026-11-16 22:00"), p("out", "2026-11-17 06:00")}, tpe)
	s := Summarize(segs, at("2026-11-16 00:00"), at("2026-11-18 00:00"), tpe)
	if s.Days[0].Hours != 8 || s.Days[1].Hours != 0 {
		t.Fatalf("unexpected %+v", s.Days)
	}
}

func TestMissingClockOut(t *testing.T) {
	segs := Pair([]Punch{
		p("in", "2026-11-16 09:00"),
		p("in", "2026-11-17 09:00"), p("out", "2026-11-17 17:00"),
		p("in", "2026-11-18 09:00"),
	}, tpe)
	s := Summarize(segs, at("2026-11-16 00:00"), at("2026-11-19 00:00"), tpe)
	if s.TotalHours != 8 || s.Incomplete != 2 || s.DaysWorked != 3 {
		t.Fatalf("unexpected %+v", s)
	}
}

func TestOrphanOutIgnored(t *testing.T) {
	segs := Pair([]Punch{p("out", "2026-11-16 06:00"), p("in", "2026-11-16 09:00"), p("out", "2026-11-16 10:00")}, tpe)
	if len(segs) != 1 || segs[0].Minutes != 60 {
		t.Fatalf("unexpected %+v", segs)
	}
}

func TestSegmentsOutsideRangeExcluded(t *testing.T) {
	segs := Pair([]Punch{
		p("in", "2026-11-15 22:00"), p("out", "2026-11-16 02:00"), // 屬於 11/15
		p("in", "2026-11-16 09:00"), p("out", "2026-11-16 10:00"),
	}, tpe)
	s := Summarize(segs, at("2026-11-16 00:00"), at("2026-11-17 00:00"), tpe)
	if s.TotalHours != 1 {
		t.Fatalf("unexpected %+v", s)
	}
}

func TestRanges(t *testing.T) {
	d := at("2026-11-18 15:00") // 週三
	cases := []struct{ period, start, end string }{
		{"day", "2026-11-18", "2026-11-19"},
		{"week", "2026-11-16", "2026-11-23"},
		{"month", "2026-11-01", "2026-12-01"},
	}
	for _, c := range cases {
		s, e := Range(c.period, d, tpe)
		if s.Format(time.DateOnly) != c.start || e.Format(time.DateOnly) != c.end {
			t.Errorf("%s: got %s..%s", c.period, s, e)
		}
	}
	// 週日應屬於前一個週一開始的那週
	s, _ := Range("week", at("2026-11-22 10:00"), tpe)
	if s.Format(time.DateOnly) != "2026-11-16" {
		t.Errorf("sunday week start = %s", s)
	}
	// 跨年的月份
	s, e := Range("month", at("2026-12-31 23:00"), tpe)
	if s.Format(time.DateOnly) != "2026-12-01" || e.Format(time.DateOnly) != "2027-01-01" {
		t.Errorf("december = %s..%s", s, e)
	}
}

func TestNextType(t *testing.T) {
	now := at("2026-11-18 18:00")
	if NextType("", time.Time{}, now) != "in" {
		t.Error("no punches should be in")
	}
	if NextType("in", at("2026-11-18 09:00"), now) != "out" {
		t.Error("after in should be out")
	}
	if NextType("out", at("2026-11-18 12:00"), now) != "in" {
		t.Error("after out should be in")
	}
	if NextType("in", at("2026-11-17 09:00"), now) != "in" {
		t.Error("stale in should allow new in")
	}
}

func TestOpenShiftNotCountedAsMissing(t *testing.T) {
	segs := Pair([]Punch{p("in", "2026-11-18 09:00")}, tpe)
	MarkOpen(segs, at("2026-11-18 12:00"))
	s := Summarize(segs, at("2026-11-18 00:00"), at("2026-11-19 00:00"), tpe)
	if !segs[0].Open || s.Incomplete != 0 || s.DaysWorked != 1 {
		t.Fatalf("unexpected %+v", s)
	}
	segs = Pair([]Punch{p("in", "2026-11-18 09:00")}, tpe)
	MarkOpen(segs, at("2026-11-19 12:00"))
	if segs[0].Open {
		t.Fatal("stale shift should not be open")
	}
}
