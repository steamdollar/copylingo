package miniapp

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"testing"
	"time"
)

func TestInitDataVerifier_Verify(t *testing.T) {
	botToken := "123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
	maxAge := 24 * time.Hour
	now := time.Now()

	v := NewInitDataVerifier(botToken, maxAge)
	v.now = func() time.Time { return now }

	// Valid data generation
	userData := `{"id":12345678,"first_name":"Test","username":"testuser"}`
	authDate := fmt.Sprintf("%d", now.Unix())
	
	params := url.Values{}
	params.Set("auth_date", authDate)
	params.Set("query_id", "AAHd_E0_AAAAAN38TT-S-F4G")
	params.Set("user", userData)
	
	// Create data_check_string
	// Pairs sorted by key: auth_date=..., query_id=..., user=...
	dataCheckString := fmt.Sprintf("auth_date=%s\nquery_id=AAHd_E0_AAAAAN38TT-S-F4G\nuser=%s", authDate, userData)
	
	secret := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	expectedHash := hex.EncodeToString(hmacSHA256(secret, []byte(dataCheckString)))
	
	initData := params.Encode() + "&hash=" + expectedHash

	t.Run("Valid init data", func(t *testing.T) {
		user, err := v.Verify(initData)
		if err != nil {
			t.Fatalf("Verify() unexpected error: %v", err)
		}
		if user.ID != 12345678 {
			t.Errorf("Verify() user.ID = %d, want 12345678", user.ID)
		}
	})

	t.Run("Missing hash", func(t *testing.T) {
		invalid := params.Encode()
		_, err := v.Verify(invalid)
		if err != ErrInitDataInvalid {
			t.Errorf("expected ErrInitDataInvalid, got %v", err)
		}
	})

	t.Run("Invalid hash", func(t *testing.T) {
		invalid := params.Encode() + "&hash=invalidhash"
		_, err := v.Verify(invalid)
		if err != ErrInitDataInvalid {
			t.Errorf("expected ErrInitDataInvalid, got %v", err)
		}
	})

	t.Run("Expired data", func(t *testing.T) {
		vOld := NewInitDataVerifier(botToken, maxAge)
		vOld.now = func() time.Time { return now.Add(25 * time.Hour) }
		
		_, err := vOld.Verify(initData)
		if err != ErrInitDataExpired {
			t.Errorf("expected ErrInitDataExpired, got %v", err)
		}
	})

	t.Run("Missing user", func(t *testing.T) {
		p := url.Values{}
		p.Set("auth_date", authDate)
		
		dcs := "auth_date=" + authDate
		h := hex.EncodeToString(hmacSHA256(secret, []byte(dcs)))
		
		_, err := v.Verify(p.Encode() + "&hash=" + h)
		if err != ErrInitDataInvalid {
			t.Errorf("expected ErrInitDataInvalid, got %v", err)
		}
	})
}
