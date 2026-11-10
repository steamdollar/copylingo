package miniapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInitDataMissing = errors.New("telegram init data is missing")
	ErrInitDataInvalid = errors.New("telegram init data is invalid")
	ErrInitDataExpired = errors.New("telegram init data is expired")
)

type TelegramUser struct {
	ID int64 `json:"id"`
}

const InitDataMaxAge = 24 * time.Hour

type InitDataVerifier struct {
	botToken string
	maxAge   time.Duration
	now      func() time.Time
}

func NewInitDataVerifier(
	botToken string,
	maxAge time.Duration,
) *InitDataVerifier {
	return &InitDataVerifier{
		botToken: botToken,
		maxAge:   maxAge,
		now:      time.Now,
	}
}

func (verifier *InitDataVerifier) Verify(initData string) (*TelegramUser, error) {
	if initData == "" {
		return nil, ErrInitDataMissing
	}
	if verifier.botToken == "" {
		return nil, fmt.Errorf("telegram bot token is empty")
	}

	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: parse query: %v",
			ErrInitDataInvalid,
			err,
		)
	}

	gotHash := values.Get("hash")
	if gotHash == "" {
		return nil, ErrInitDataInvalid
	}
	values.Del("hash")

	var pairs []string
	for key, fieldValues := range values {
		if len(fieldValues) == 0 {
			continue
		}
		pairs = append(
			pairs,
			key+"="+fieldValues[0],
		)
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(
		pairs,
		"\n",
	)

	secret := hmacSHA256(
		[]byte("WebAppData"),
		[]byte(verifier.botToken),
	)
	expected := hmacSHA256(
		secret,
		[]byte(dataCheckString),
	)
	got, err := hex.DecodeString(gotHash)
	if err != nil {
		return nil, ErrInitDataInvalid
	}
	if !hmac.Equal(
		got,
		expected,
	) {
		return nil, ErrInitDataInvalid
	}

	if verifier.maxAge > 0 {
		authDateRaw := values.Get("auth_date")
		authDate, err := strconv.ParseInt(
			authDateRaw,
			10,
			64,
		)
		if err != nil {
			return nil, ErrInitDataInvalid
		}
		if verifier.now().Sub(time.Unix(
			authDate,
			0,
		)) > verifier.maxAge {
			return nil, ErrInitDataExpired
		}
	}

	userRaw := values.Get("user")
	if userRaw == "" {
		return nil, ErrInitDataInvalid
	}
	var user TelegramUser
	if err := json.Unmarshal(
		[]byte(userRaw),
		&user,
	); err != nil {
		return nil, fmt.Errorf(
			"%w: parse user: %v",
			ErrInitDataInvalid,
			err,
		)
	}
	if user.ID == 0 {
		return nil, ErrInitDataInvalid
	}

	return &user, nil
}

func hmacSHA256(
	key,
	data []byte,
) []byte {
	hmacHasher := hmac.New(
		sha256.New,
		key,
	)
	hmacHasher.Write(data)
	return hmacHasher.Sum(nil)
}
