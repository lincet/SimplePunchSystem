package auth

import (
	"testing"
	"time"
)

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "secret123") || CheckPassword(h, "wrong") {
		t.Fatal("password check failed")
	}
	if ValidatePassword("short") == nil || ValidatePassword("密碼密碼密碼密碼") != nil {
		t.Fatal("validation wrong")
	}
}

func TestToken(t *testing.T) {
	tok, hash, err := NewToken()
	if err != nil || tok == "" || HashToken(tok) != hash {
		t.Fatal("token hash mismatch")
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if l.Blocked("k") {
			t.Fatal("blocked too early")
		}
		l.Fail("k")
	}
	if !l.Blocked("k") {
		t.Fatal("should be blocked")
	}
	l.Reset("k")
	if l.Blocked("k") {
		t.Fatal("reset failed")
	}
}
