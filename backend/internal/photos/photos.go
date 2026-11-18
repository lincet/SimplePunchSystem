// Package photos 儲存與讀取打卡照片。
package photos

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MaxSize = 5 << 20 // 5MB

var (
	ErrTooLarge   = errors.New("照片太大（上限 5MB）")
	ErrNotAnImage = errors.New("檔案不是 JPEG 或 PNG 圖片")
	ErrEmpty      = errors.New("沒有照片")
)

type Store struct{ root string }

func New(dataDir string) *Store { return &Store{root: filepath.Join(dataDir, "photos")} }

// Save 驗證並寫入照片，回傳相對路徑（存在 DB）。
func (s *Store) Save(r io.Reader, at time.Time) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxSize+1))
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", ErrEmpty
	}
	if len(data) > MaxSize {
		return "", ErrTooLarge
	}
	ext := sniff(data)
	if ext == "" {
		return "", ErrNotAnImage
	}
	id := make([]byte, 16)
	rand.Read(id)
	rel := filepath.Join(at.UTC().Format("2006"), at.UTC().Format("01"), hex.EncodeToString(id)+ext)
	full := filepath.Join(s.root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// Path 把 DB 中的相對路徑轉成檔案路徑，並防止路徑跳脫。
func (s *Store) Path(rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", errors.New("invalid photo path")
	}
	return filepath.Join(s.root, clean), nil
}

func sniff(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return ".jpg"
	case bytes.HasPrefix(b, []byte{0x89, 'P', 'N', 'G'}):
		return ".png"
	}
	return ""
}
