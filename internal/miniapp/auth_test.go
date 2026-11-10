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

	verifier := NewInitDataVerifier(
		botToken,
		maxAge,
	)
	verifier.now = func() time.Time { return now }

	// Valid data generation
	userData := `{"id":12345678,"first_name":"Test","username":"testuser"}`
	authDate := fmt.Sprintf(
		"%d",
		now.Unix(),
	)

	queryParameters := url.Values{}
	queryParameters.Set(
		"auth_date",
		authDate,
	)
	queryParameters.Set(
		"query_id",
		"AAHd_E0_AAAAAN38TT-S-F4G",
	)
	queryParameters.Set(
		"user",
		userData,
	)

	// Create data_check_string
	// Pairs sorted by key: auth_date=..., query_id=..., user=...
	dataCheckString := fmt.Sprintf(
		"auth_date=%s\nquery_id=AAHd_E0_AAAAAN38TT-S-F4G\nuser=%s",
		authDate,
		userData,
	)

	secret := hmacSHA256(
		[]byte("WebAppData"),
		[]byte(botToken),
	)
	expectedHash := hex.EncodeToString(hmacSHA256(
		secret,
		[]byte(dataCheckString),
	))

	initData := queryParameters.Encode() + "&hash=" + expectedHash

	t.Run(
		"Valid init data",
		func(t *testing.T) {
			user, err := verifier.Verify(initData)
			if err != nil {
				t.Fatalf(
					"Verify() unexpected error: %v",
					err,
				)
			}
			if user.ID != 12345678 {
				t.Errorf(
					"Verify() user.ID = %d, want 12345678",
					user.ID,
				)
			}
		},
	)

	t.Run(
		"Missing hash",
		func(t *testing.T) {
			invalid := queryParameters.Encode()
			_, err := verifier.Verify(invalid)
			if err != ErrInitDataInvalid {
				t.Errorf(
					"expected ErrInitDataInvalid, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"Invalid hash",
		func(t *testing.T) {
			invalid := queryParameters.Encode() + "&hash=invalidhash"
			_, err := verifier.Verify(invalid)
			if err != ErrInitDataInvalid {
				t.Errorf(
					"expected ErrInitDataInvalid, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"Expired data",
		func(t *testing.T) {
			oldVerifier := NewInitDataVerifier(
				botToken,
				maxAge,
			)
			oldVerifier.now = func() time.Time { return now.Add(25 * time.Hour) }

			_, err := oldVerifier.Verify(initData)
			if err != ErrInitDataExpired {
				t.Errorf(
					"expected ErrInitDataExpired, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"Missing user",
		func(t *testing.T) {
			form := url.Values{}
			form.Set(
				"auth_date",
				authDate,
			)

			dataCheckString := "auth_date=" + authDate
			hash := hex.EncodeToString(hmacSHA256(
				secret,
				[]byte(dataCheckString),
			))

			_, err := verifier.Verify(form.Encode() + "&hash=" + hash)
			if err != ErrInitDataInvalid {
				t.Errorf(
					"expected ErrInitDataInvalid, got %v",
					err,
				)
			}
		},
	)
}
