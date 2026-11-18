// Package config 從環境變數讀取伺服器設定。
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port          string
	DataDir       string
	TZName        string
	Loc           *time.Location
	AdminUsername string
	AdminPassword string
	StaticDir     string // 本機開發時由 Go 直接提供前端檔案；正式環境前端在 Netlify，留空
	CookieSecure  bool
	DemoSeed      bool
}

func Load() (*Config, error) {
	c := &Config{
		Port:          env("PORT", "8080"),
		DataDir:       env("DATA_DIR", "./data"),
		TZName:        env("TZ_NAME", "Asia/Taipei"),
		AdminUsername: env("ADMIN_USERNAME", "admin"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		StaticDir:     os.Getenv("STATIC_DIR"),
		CookieSecure:  envBool("COOKIE_SECURE", false),
		DemoSeed:      envBool("DEMO_SEED", false),
	}
	loc, err := time.LoadLocation(c.TZName)
	if err != nil {
		return nil, fmt.Errorf("載入時區 %q 失敗: %w", c.TZName, err)
	}
	c.Loc = loc
	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
