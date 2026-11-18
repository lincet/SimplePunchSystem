// SimplePunchSystem 打卡系統後端。
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // 內嵌時區資料，主機沒有 tzdata 也能用 Asia/Taipei

	"simplepunch/internal/api"
	"simplepunch/internal/config"
	"simplepunch/internal/db"
	"simplepunch/internal/photos"
	"simplepunch/internal/seed"
	"simplepunch/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	sqlDB, err := db.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("開啟資料庫失敗: %v", err)
	}
	defer sqlDB.Close()

	st := store.New(sqlDB)
	ph := photos.New(cfg.DataDir)

	if err := seed.EnsureAdmin(st, cfg); err != nil {
		log.Fatalf("建立管理者失敗: %v", err)
	}
	srv := api.New(cfg, st, ph)
	if cfg.DemoSeed {
		if err := seed.Demo(st, ph, cfg, time.Now()); err != nil {
			log.Fatalf("建立 demo 資料失敗: %v", err)
		}
		srv.DemoAccounts = seed.DemoAccounts(cfg)
	}

	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		t := time.NewTicker(6 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				st.PurgeExpiredSessions()
			}
		}
	}()

	go func() {
		log.Printf("打卡系統啟動於 :%s（時區 %s，demo=%v）", cfg.Port, cfg.TZName, cfg.DemoSeed)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Println("正在關閉伺服器…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpSrv.Shutdown(shutdownCtx)
}
