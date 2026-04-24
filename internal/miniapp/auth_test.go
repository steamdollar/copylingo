package miniapp

import (
	"encoding/hex"
	"net/url"
	"testing"
	"time"
)

func TestInitDataVerifierVerify(t *testing.T) {
	token := "123456:test-token"
	authDate := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	initData := signedInitData(t, token, map[string]string{
		"auth_date": "1776945600",
		"query_id":  "query-1",
		"user":      `{"id":42,"first_name":"Tester"}`,
	})

	verifier := NewInitDataVerifier(token, 24*time.Hour)
	verifier.now = func() time.Time { return authDate.Add(time.Hour) }

	user, err := verifier.Verify(initData)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if user.ID != 42 {
		t.Fatalf("Verify() user ID = %d, want 42", user.ID)
	}
}

func TestInitDataVerifierRejectsTamperedData(t *testing.T) {
	token := "123456:test-token"
	initData := signedInitData(t, token, map[string]string{
		"auth_date": "1776945600",
		"user":      `{"id":42}`,
	})
	initData += "&extra=tampered"

	verifier := NewInitDataVerifier(token, 24*time.Hour)
	verifier.now = func() time.Time {
		return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	}

	if _, err := verifier.Verify(initData); err == nil {
		t.Fatal("Verify() error = nil, want invalid signature")
	}
}

func signedInitData(t *testing.T, token string, fields map[string]string) string {
	t.Helper()

	keys := []string{"auth_date", "query_id", "user"}
	dataCheckString := ""
	first := true
	for _, key := range keys {
		value, ok := fields[key]
		if !ok {
			continue
		}
		if !first {
			dataCheckString += "\n"
		}
		dataCheckString += key + "=" + value
		first = false
	}

	secret := hmacSHA256([]byte("WebAppData"), []byte(token))
	hash := hmacSHA256(secret, []byte(dataCheckString))

	values := url.Values{}
	for key, value := range fields {
		values.Set(key, value)
	}
	values.Set("hash", hex.EncodeToString(hash))
	return values.Encode()
}
