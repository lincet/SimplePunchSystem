package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/xuri/excelize/v2"

	"simplepunch/internal/hours"
)

var weekdayZh = [...]string{"日", "一", "二", "三", "四", "五", "六"}

var detailHeader = []string{"員工編號", "姓名", "日期", "星期", "上班時間", "下班時間", "時數", "備註"}

// detailRows 把報表攤平成「每一段上班」一列。
func (s *Server) detailRows(reports []UserReport) [][]any {
	var rows [][]any
	for _, rep := range reports {
		for _, day := range rep.Summary.Days {
			for _, seg := range day.Segments {
				d, _ := time.ParseInLocation(time.DateOnly, day.Date, s.cfg.Loc)
				in := seg.In.In(s.cfg.Loc)
				out, hrs, note := "", any(""), ""
				if seg.Out != nil {
					o := seg.Out.In(s.cfg.Loc)
					out = o.Format("15:04")
					if o.Format(time.DateOnly) != day.Date {
						out = o.Format("01/02 15:04") // 跨夜
					}
					hrs = hours.Round2(seg.Minutes / 60)
				} else if seg.Open {
					note = "上班中"
				} else {
					note = "未打下班卡"
				}
				rows = append(rows, []any{rep.User.EmployeeNo, rep.User.Name, day.Date, weekdayZh[d.Weekday()],
					in.Format("15:04"), out, hrs, note})
			}
		}
	}
	return rows
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	start, end, err := s.parseDateRange(r, 30)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_range", err.Error())
		return
	}
	reports, err := s.buildReports(queryUserID(r), start, end)
	if err != nil {
		serverError(w, err)
		return
	}
	base := fmt.Sprintf("打卡紀錄_%s_%s", start.Format("20060102"), end.AddDate(0, 0, -1).Format("20060102"))
	rows := s.detailRows(reports)

	if r.URL.Query().Get("format") == "csv" {
		setDownload(w, base+".csv", "text/csv; charset=utf-8")
		w.Write([]byte("\xEF\xBB\xBF")) // UTF-8 BOM，讓 Excel 正確顯示中文
		cw := csv.NewWriter(w)
		cw.Write(detailHeader)
		for _, row := range rows {
			rec := make([]string, len(row))
			for i, v := range row {
				rec[i] = fmt.Sprint(v)
			}
			cw.Write(rec)
		}
		cw.Flush()
		return
	}

	f := excelize.NewFile()
	defer f.Close()
	bold, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#E8EEF7"}},
	})

	const summary = "工時統計"
	f.SetSheetName("Sheet1", summary)
	period := fmt.Sprintf("期間：%s ~ %s", start.Format(time.DateOnly), end.AddDate(0, 0, -1).Format(time.DateOnly))
	f.SetCellValue(summary, "A1", period)
	f.SetSheetRow(summary, "A2", &[]any{"員工編號", "姓名", "職稱", "出勤天數", "總時數", "未打下班卡次數"})
	f.SetCellStyle(summary, "A2", "F2", bold)
	var total float64
	for i, rep := range reports {
		f.SetSheetRow(summary, fmt.Sprintf("A%d", i+3), &[]any{rep.User.EmployeeNo, rep.User.Name, rep.User.Title,
			rep.Summary.DaysWorked, rep.Summary.TotalHours, rep.Summary.Incomplete})
		total += rep.Summary.TotalHours
	}
	last := len(reports) + 3
	f.SetSheetRow(summary, fmt.Sprintf("A%d", last), &[]any{"", "合計", "", "", hours.Round2(total), ""})
	f.SetColWidth(summary, "A", "F", 14)

	const detail = "打卡明細"
	f.NewSheet(detail)
	f.SetSheetRow(detail, "A1", &detailHeader)
	f.SetCellStyle(detail, "A1", "H1", bold)
	for i, row := range rows {
		f.SetSheetRow(detail, fmt.Sprintf("A%d", i+2), &row)
	}
	f.SetColWidth(detail, "A", "H", 13)
	f.SetPanes(detail, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})

	setDownload(w, base+".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	if err := f.Write(w); err != nil {
		serverError(w, err)
	}
}

func setDownload(w http.ResponseWriter, filename, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="attendance%s"; filename*=UTF-8''%s`, extOf(filename), url.PathEscape(filename)))
}

func extOf(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[i:]
		}
	}
	return ""
}
