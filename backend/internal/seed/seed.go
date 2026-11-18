// Package seed 建立老闆帳號與 demo 示範資料。
package seed

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"math"
	"math/rand"
	"time"

	"simplepunch/internal/api"
	"simplepunch/internal/auth"
	"simplepunch/internal/config"
	"simplepunch/internal/hours"
	"simplepunch/internal/photos"
	"simplepunch/internal/store"
)

const demoPassword = "demo1234"

// EnsureAdmin 在沒有任何管理者時建立老闆帳號。
func EnsureAdmin(st *store.Store, cfg *config.Config) error {
	n, err := st.CountAdmins()
	if err != nil || n > 0 {
		return err
	}
	pw := cfg.AdminPassword
	generated := false
	if pw == "" {
		if cfg.DemoSeed {
			pw = demoPassword
		} else {
			pw = auth.RandomPassword(10)
			generated = true
		}
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return err
	}
	u := &store.User{
		Role: "admin", Username: api.NormalizeUsername(cfg.AdminUsername), Name: "老闆", Gender: "other",
		PasswordHash: hash, MustChangePassword: !cfg.DemoSeed, Active: true,
	}
	if err := st.CreateUser(u); err != nil {
		return err
	}
	if generated {
		log.Printf("已建立管理者帳號 %q，初始密碼：%s（首次登入需更改）", u.Username, pw)
	} else {
		log.Printf("已建立管理者帳號 %q", u.Username)
	}
	return nil
}

type demoEmployee struct {
	name, phone, gender, no, title, hire string
	password                             string
	mustChange                           bool
	hue                                  float64
	punches                              bool
}

var demoEmployees = []demoEmployee{
	{"王小明", "0911000001", "male", "E001", "門市店員", "2024-03-01", demoPassword, false, 210, true},
	{"李美華", "0911000002", "female", "E002", "會計", "2023-08-15", demoPassword, false, 330, true},
	{"陳志強", "0911000003", "male", "E003", "倉管", "2025-01-06", demoPassword, false, 140, true},
	{"林新人", "0911000004", "female", "E004", "助理", "2026-11-01", "init1234", true, 40, false},
}

// DemoAccounts 回傳登入頁要顯示的示範帳號。
func DemoAccounts(cfg *config.Config) []api.DemoAccount {
	adminPw := demoPassword
	if cfg.AdminPassword != "" {
		adminPw = "（由管理者設定）"
	}
	accts := []api.DemoAccount{{Label: "老闆", Username: api.NormalizeUsername(cfg.AdminUsername), Password: adminPw}}
	for _, e := range demoEmployees {
		label := "員工 " + e.name
		if e.mustChange {
			label += "（首次登入需改密碼）"
		}
		accts = append(accts, api.DemoAccount{Label: label, Username: e.phone, Password: e.password})
	}
	return accts
}

// Demo 在資料庫沒有任何員工時，建立示範員工與過去兩週的打卡紀錄。
func Demo(st *store.Store, ph *photos.Store, cfg *config.Config, now time.Time) error {
	existing, err := st.ListEmployees(true)
	if err != nil || len(existing) > 0 {
		return err
	}
	rng := rand.New(rand.NewSource(42))
	today := hours.StartOfDay(now, cfg.Loc)
	for i, e := range demoEmployees {
		hash, err := auth.HashPassword(e.password)
		if err != nil {
			return err
		}
		u := &store.User{
			Role: "employee", Username: e.phone, Name: e.name, Phone: e.phone, Gender: e.gender,
			EmployeeNo: e.no, Title: e.title, HireDate: e.hire, PasswordHash: hash,
			MustChangePassword: e.mustChange, Active: true,
		}
		if err := st.CreateUser(u); err != nil {
			return err
		}
		if !e.punches {
			continue
		}
		photo := placeholderPhoto(e.hue)
		for d := 14; d >= 1; d-- {
			day := today.AddDate(0, 0, -d)
			if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
				continue
			}
			in := day.Add(8*time.Hour + 45*time.Minute + time.Duration(rng.Intn(30))*time.Minute)
			out := day.Add(17*time.Hour + 55*time.Minute + time.Duration(rng.Intn(50))*time.Minute)
			if err := addPunch(st, ph, u.ID, "in", in, photo); err != nil {
				return err
			}
			if i == 2 && d == 3 {
				continue // 示範「忘記打下班卡」
			}
			if err := addPunch(st, ph, u.ID, "out", out, photo); err != nil {
				return err
			}
		}
	}
	log.Printf("已建立 demo 示範資料（%d 位員工）", len(demoEmployees))
	return nil
}

func addPunch(st *store.Store, ph *photos.Store, userID int64, typ string, at time.Time, photo []byte) error {
	rel, err := ph.Save(bytes.NewReader(photo), at)
	if err != nil {
		return fmt.Errorf("save demo photo: %w", err)
	}
	return st.CreatePunch(&store.Punch{UserID: userID, Type: typ, PunchedAt: at, PhotoPath: rel, UserAgent: "demo-seed"})
}

// placeholderPhoto 產生一張簡單的人像剪影 JPEG，作為示範打卡照片。
func placeholderPhoto(hue float64) []byte {
	const w, h = 300, 400
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bgTop, bgBottom := hsl(hue, 0.45, 0.78), hsl(hue, 0.45, 0.58)
	fg := hsl(hue, 0.35, 0.95)
	for y := 0; y < h; y++ {
		t := float64(y) / h
		row := lerp(bgTop, bgBottom, t)
		for x := 0; x < w; x++ {
			c := row
			dx, dy := float64(x-w/2), float64(y-150)
			if dx*dx+dy*dy < 62*62 { // 頭
				c = fg
			}
			sx, sy := float64(x-w/2)/120, float64(y-400)/150
			if y > 230 && sx*sx+sy*sy < 1 { // 肩膀
				c = fg
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	return buf.Bytes()
}

func lerp(a, b color.RGBA, t float64) color.RGBA {
	m := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 255}
}

func hsl(h, s, l float64) color.RGBA {
	c := (1 - math.Abs(2*l-1)) * s
	hp := math.Mod(h/60, 6)
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	var r, g, b float64
	switch {
	case hp < 1:
		r, g = c, x
	case hp < 2:
		r, g = x, c
	case hp < 3:
		g, b = c, x
	case hp < 4:
		g, b = x, c
	case hp < 5:
		r, b = x, c
	default:
		r, b = c, x
	}
	m := l - c/2
	to := func(v float64) uint8 { return uint8(math.Round((v + m) * 255)) }
	return color.RGBA{to(r), to(g), to(b), 255}
}
