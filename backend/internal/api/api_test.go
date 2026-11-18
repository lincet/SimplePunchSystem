package api_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"simplepunch/internal/api"
	"simplepunch/internal/config"
	"simplepunch/internal/db"
	"simplepunch/internal/photos"
	"simplepunch/internal/seed"
	"simplepunch/internal/store"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	loc, _ := time.LoadLocation("Asia/Taipei")
	cfg := &config.Config{DataDir: dir, TZName: "Asia/Taipei", Loc: loc, AdminUsername: "admin", AdminPassword: "adminpass1"}
	d, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	st := store.New(d)
	if err := seed.EnsureAdmin(st, cfg); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(api.New(cfg, st, photos.New(dir)).Handler())
	t.Cleanup(ts.Close)
	return ts
}

type client struct {
	t    *testing.T
	base string
	c    *http.Client
}

func newClient(t *testing.T, ts *httptest.Server) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: ts.URL, c: &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, c.base+path, rd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "fetch")
	return c.send(req)
}

func (c *client) send(req *http.Request) (int, map[string]any) {
	c.t.Helper()
	resp, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (c *client) punch(typ string, withPhoto bool) (int, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("type", typ)
	if withPhoto {
		fw, _ := mw.CreateFormFile("photo", "p.jpg")
		jpeg.Encode(fw, image.NewRGBA(image.Rect(0, 0, 10, 10)), nil)
	}
	mw.Close()
	req, _ := http.NewRequest("POST", c.base+"/api/punch", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Requested-With", "fetch")
	return c.send(req)
}

func expect(t *testing.T, got, want int, body map[string]any, step string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: status %d, want %d, body %v", step, got, want, body)
	}
}

func TestFullFlow(t *testing.T) {
	ts := newTestServer(t)
	boss := newClient(t, ts)

	code, body := boss.do("POST", "/api/auth/login", map[string]string{"username": "admin", "password": "wrong"})
	expect(t, code, 401, body, "wrong password")

	code, body = boss.do("POST", "/api/auth/login", map[string]string{"username": "admin", "password": "adminpass1"})
	expect(t, code, 200, body, "boss login")
	if body["mustChangePassword"] != true {
		t.Fatal("admin should need to change password")
	}
	code, body = boss.do("GET", "/api/admin/users", nil)
	expect(t, code, 403, body, "admin before password change")

	code, body = boss.do("POST", "/api/me/password", map[string]string{"oldPassword": "adminpass1", "newPassword": "newadmin99"})
	expect(t, code, 200, body, "boss change password")

	code, body = boss.do("POST", "/api/admin/users", map[string]any{
		"name": "測試員工", "phone": "0912-345-678", "gender": "female", "password": "short",
	})
	expect(t, code, 400, body, "weak initial password")
	code, body = boss.do("POST", "/api/admin/users", map[string]any{
		"name": "測試員工", "phone": "0912-345-678", "gender": "female", "password": "init12345",
	})
	expect(t, code, 201, body, "create employee")
	empID := int(body["id"].(float64))
	code, body = boss.do("POST", "/api/admin/users", map[string]any{
		"name": "重複", "phone": "0912345678", "gender": "male", "password": "init12345",
	})
	expect(t, code, 409, body, "duplicate phone")

	emp := newClient(t, ts)
	code, body = emp.do("POST", "/api/auth/login", map[string]string{"username": "0912345678", "password": "init12345"})
	expect(t, code, 200, body, "employee login")
	code, body = emp.punch("in", true)
	expect(t, code, 403, body, "punch before password change")

	code, body = emp.do("POST", "/api/me/password", map[string]string{"oldPassword": "init12345", "newPassword": "mine12345"})
	expect(t, code, 200, body, "employee change password")

	code, body = emp.do("GET", "/api/admin/users", nil)
	expect(t, code, 403, body, "employee calling admin api")
	code, body = emp.do("PUT", "/api/admin/users/1", map[string]any{"name": "x"})
	expect(t, code, 403, body, "employee editing profile")

	code, body = emp.punch("in", false)
	expect(t, code, 400, body, "punch without photo")
	code, body = emp.punch("out", true)
	expect(t, code, 409, body, "clock out before clock in")
	code, body = emp.punch("in", true)
	expect(t, code, 201, body, "clock in")
	if body["next"] != "out" {
		t.Fatalf("next should be out: %v", body)
	}
	code, body = emp.punch("in", true)
	expect(t, code, 409, body, "double clock in")
	code, body = emp.punch("out", true)
	expect(t, code, 201, body, "clock out")

	code, body = boss.do("GET", "/api/admin/hours?period=day", nil)
	expect(t, code, 200, body, "hours")
	reports := body["reports"].([]any)
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %v", body)
	}
	segs := reports[0].(map[string]any)["summary"].(map[string]any)["days"].([]any)[0].(map[string]any)["segments"].([]any)
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %v", segs)
	}

	// 照片權限：本人與老闆可以看
	resp, _ := emp.c.Get(ts.URL + "/api/photos/1")
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") {
		t.Fatalf("employee photo: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	resp.Body.Close()

	// 匯出
	for _, f := range []string{"xlsx", "csv"} {
		resp, _ := boss.c.Get(ts.URL + "/api/admin/export?format=" + f)
		if resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Content-Disposition"), "attachment") {
			t.Fatalf("export %s: %d", f, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// 停用員工後，其 session 立即失效
	code, body = boss.do("PUT", "/api/admin/users/"+strconv.Itoa(empID), map[string]any{
		"name": "測試員工", "phone": "0912345678", "gender": "female", "active": false,
	})
	expect(t, code, 200, body, "deactivate")
	code, body = emp.do("GET", "/api/me", nil)
	expect(t, code, 401, body, "deactivated session")

	// CSRF：沒有自訂標頭的 POST 會被拒絕
	req, _ := http.NewRequest("POST", ts.URL+"/api/auth/login", strings.NewReader(`{}`))
	code, body = boss.send(req)
	expect(t, code, 403, body, "csrf header missing")
}
