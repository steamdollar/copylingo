package redisstore

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/model"
)

type fakeInteractionsRedis struct {
	redis.Cmdable
	values     map[string]string
	expiration map[string]time.Duration
	getErr     error
	getDelErr  error
	setErr     error
	delErr     error
	getKeys    []string
	getDelKeys []string
	setKeys    []string
	delCalls   [][]string
}

func newFakeInteractionsRedis() *fakeInteractionsRedis {
	return &fakeInteractionsRedis{
		values:     make(map[string]string),
		expiration: make(map[string]time.Duration),
	}
}

func (f *fakeInteractionsRedis) Get(
	_ context.Context,
	key string,
) *redis.StringCmd {
	f.getKeys = append(
		f.getKeys,
		key,
	)
	if f.getErr != nil {
		return redis.NewStringResult(
			"",
			f.getErr,
		)
	}
	value, ok := f.values[key]
	if !ok {
		return redis.NewStringResult(
			"",
			redis.Nil,
		)
	}
	return redis.NewStringResult(
		value,
		nil,
	)
}

func (f *fakeInteractionsRedis) GetDel(
	_ context.Context,
	key string,
) *redis.StringCmd {
	f.getDelKeys = append(
		f.getDelKeys,
		key,
	)
	if f.getDelErr != nil {
		return redis.NewStringResult(
			"",
			f.getDelErr,
		)
	}
	value, ok := f.values[key]
	if !ok {
		return redis.NewStringResult(
			"",
			redis.Nil,
		)
	}
	delete(
		f.values,
		key,
	)
	delete(
		f.expiration,
		key,
	)
	return redis.NewStringResult(
		value,
		nil,
	)
}

func (f *fakeInteractionsRedis) Set(
	_ context.Context,
	key string,
	value any,
	expiration time.Duration,
) *redis.StatusCmd {
	f.setKeys = append(
		f.setKeys,
		key,
	)
	if f.setErr != nil {
		return redis.NewStatusResult(
			"",
			f.setErr,
		)
	}
	switch value := value.(type) {
	case []byte:
		f.values[key] = string(value)
	default:
		f.values[key] = fmt.Sprint(value)
	}
	f.expiration[key] = expiration
	return redis.NewStatusResult(
		"OK",
		nil,
	)
}

func (f *fakeInteractionsRedis) Del(
	_ context.Context,
	keys ...string,
) *redis.IntCmd {
	f.delCalls = append(
		f.delCalls,
		append(
			[]string(nil),
			keys...,
		),
	)
	if f.delErr != nil {
		return redis.NewIntResult(
			0,
			f.delErr,
		)
	}
	var deleted int64
	for _, key := range keys {
		if _, ok := f.values[key]; ok {
			delete(
				f.values,
				key,
			)
			delete(
				f.expiration,
				key,
			)
			deleted++
		}
	}
	return redis.NewIntResult(
		deleted,
		nil,
	)
}

func assertInteractionSet(
	t *testing.T,
	rdb *fakeInteractionsRedis,
	key,
	value string,
	ttl time.Duration,
) {
	t.Helper()
	if got := rdb.values[key]; got != value {
		t.Fatalf(
			"Redis value for %q = %q, want %q",
			key,
			got,
			value,
		)
	}
	if got := rdb.expiration[key]; got != ttl {
		t.Fatalf(
			"Redis TTL for %q = %s, want %s",
			key,
			got,
			ttl,
		)
	}
}

func TestLLMPendingFormatsAndConsumesOnce(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeInteractionsRedis()
	store := NewInteractions(rdb)
	cases := []struct {
		userID int64
		input  model.PendingLLMInput
		value  string
	}{
		{userID: 10, input: model.PendingLLMInput{Kind: model.PendingLLMPlain}, value: "1"},
		{
			userID: 11,
			input:  model.PendingLLMInput{Kind: model.PendingLLMQuizQuestion, SessionID: 21, QuestionID: 3},
			value:  "q:21:3",
		},
		{
			userID: 12,
			input:  model.PendingLLMInput{Kind: model.PendingLLMStudyMaterial, SessionID: 22, MaterialOrder: 4},
			value:  "study:22:4",
		},
	}

	for _, tc := range cases {
		key := fmt.Sprintf(
			"user:%d:llm_pending",
			tc.userID,
		)
		if err := store.SetLLMPending(
			ctx,
			tc.userID,
			tc.input,
		); err != nil {
			t.Fatalf(
				"SetLLMPending(%d) error = %v",
				tc.userID,
				err,
			)
		}
		assertInteractionSet(
			t,
			rdb,
			key,
			tc.value,
			10*time.Minute,
		)

		got, found, err := store.TakeLLMPending(
			ctx,
			tc.userID,
		)
		if err != nil || !found || got != tc.input {
			t.Fatalf(
				"TakeLLMPending(%d) = %+v, %t, %v; want %+v, true, nil",
				tc.userID,
				got,
				found,
				err,
				tc.input,
			)
		}
		if _, found, err := store.TakeLLMPending(
			ctx,
			tc.userID,
		); err != nil || found {
			t.Fatalf(
				"second TakeLLMPending(%d) = found %t, error %v; want missing",
				tc.userID,
				found,
				err,
			)
		}
	}

	malformedKey := "user:13:llm_pending"
	rdb.values[malformedKey] = "q:bad:3"
	got, found, err := store.TakeLLMPending(
		ctx,
		13,
	)
	if err != nil || !found || got != (model.PendingLLMInput{}) {
		t.Fatalf(
			"TakeLLMPending(malformed) = %+v, %t, %v; want zero input, consumed, nil",
			got,
			found,
			err,
		)
	}
	if _, found, err := store.TakeLLMPending(
		ctx,
		13,
	); err != nil || found {
		t.Fatalf(
			"TakeLLMPending(malformed second read) found %t, error %v; want missing",
			found,
			err,
		)
	}
	wantGetDelKeys := []string{
		"user:10:llm_pending", "user:10:llm_pending",
		"user:11:llm_pending", "user:11:llm_pending",
		"user:12:llm_pending", "user:12:llm_pending",
		malformedKey, malformedKey,
	}
	if !reflect.DeepEqual(
		rdb.getDelKeys,
		wantGetDelKeys,
	) {
		t.Fatalf(
			"pending GetDel keys = %v, want %v",
			rdb.getDelKeys,
			wantGetDelKeys,
		)
	}
}

func TestActiveQuestionAndClearInputContracts(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeInteractionsRedis()
	store := NewInteractions(rdb)
	chatID, userID := int64(123), int64(456)
	activeKey, pendingKey := "user:123:active_question", "user:456:llm_pending"
	question := model.ActiveQuestionRef{SessionID: 51, QuestionIndex: 2}

	if err := store.SetActiveQuestion(
		ctx,
		chatID,
		question,
	); err != nil {
		t.Fatalf(
			"SetActiveQuestion() error = %v",
			err,
		)
	}
	assertInteractionSet(
		t,
		rdb,
		activeKey,
		"51:2",
		time.Hour,
	)
	got, err := store.GetActiveQuestion(
		ctx,
		chatID,
	)
	if err != nil || got == nil || *got != question {
		t.Fatalf(
			"GetActiveQuestion() = %+v, %v; want %+v, nil",
			got,
			err,
			question,
		)
	}
	if len(rdb.getKeys) != 1 || rdb.getKeys[0] != activeKey || len(rdb.getDelKeys) != 0 {
		t.Fatalf(
			"active question reads: GET %v, GETDEL %v; want GET only",
			rdb.getKeys,
			rdb.getDelKeys,
		)
	}
	if err := store.DeleteActiveQuestion(
		ctx,
		chatID,
	); err != nil {
		t.Fatalf(
			"DeleteActiveQuestion() error = %v",
			err,
		)
	}
	if !reflect.DeepEqual(
		rdb.delCalls[len(rdb.delCalls)-1],
		[]string{activeKey},
	) {
		t.Fatalf(
			"DeleteActiveQuestion() keys = %v, want [%s]",
			rdb.delCalls[len(rdb.delCalls)-1],
			activeKey,
		)
	}

	if err := store.SetActiveQuestion(
		ctx,
		chatID,
		question,
	); err != nil {
		t.Fatalf(
			"SetActiveQuestion() before ClearInput error = %v",
			err,
		)
	}
	if err := store.SetLLMPending(
		ctx,
		userID,
		model.PendingLLMInput{},
	); err != nil {
		t.Fatalf(
			"SetLLMPending() before ClearInput error = %v",
			err,
		)
	}
	if err := store.ClearInput(
		ctx,
		chatID,
		&userID,
	); err != nil {
		t.Fatalf(
			"ClearInput(with user) error = %v",
			err,
		)
	}
	if !reflect.DeepEqual(
		rdb.delCalls[len(rdb.delCalls)-1],
		[]string{activeKey, pendingKey},
	) {
		t.Fatalf(
			"ClearInput(with user) keys = %v, want active question and pending keys",
			rdb.delCalls[len(rdb.delCalls)-1],
		)
	}
	if _, exists := rdb.values[activeKey]; exists {
		t.Fatalf(
			"ClearInput(with user) left %q",
			activeKey,
		)
	}
	if _, exists := rdb.values[pendingKey]; exists {
		t.Fatalf(
			"ClearInput(with user) left %q",
			pendingKey,
		)
	}

	if err := store.SetActiveQuestion(
		ctx,
		chatID,
		question,
	); err != nil {
		t.Fatalf(
			"SetActiveQuestion() before ClearInput without user error = %v",
			err,
		)
	}
	if err := store.SetLLMPending(
		ctx,
		userID,
		model.PendingLLMInput{},
	); err != nil {
		t.Fatalf(
			"SetLLMPending() before ClearInput without user error = %v",
			err,
		)
	}
	if err := store.ClearInput(
		ctx,
		chatID,
		nil,
	); err != nil {
		t.Fatalf(
			"ClearInput(without user) error = %v",
			err,
		)
	}
	if !reflect.DeepEqual(
		rdb.delCalls[len(rdb.delCalls)-1],
		[]string{activeKey},
	) {
		t.Fatalf(
			"ClearInput(without user) keys = %v, want only active question key",
			rdb.delCalls[len(rdb.delCalls)-1],
		)
	}
	if _, ok := rdb.values[pendingKey]; !ok {
		t.Fatal("ClearInput(without user) deleted the user pending key")
	}
}

func TestWordOrderDraftFormatAndCorruptCleanup(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeInteractionsRedis()
	store := NewInteractions(rdb)
	key := "session:12:word_order:4:draft"
	want := []int{2, 0, 1}

	if err := store.SetWordOrderDraft(
		ctx,
		12,
		4,
		want,
	); err != nil {
		t.Fatalf(
			"SetWordOrderDraft() error = %v",
			err,
		)
	}
	assertInteractionSet(
		t,
		rdb,
		key,
		"[2,0,1]",
		24*time.Hour,
	)
	got, err := store.GetWordOrderDraft(
		ctx,
		12,
		4,
	)
	if err != nil || !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf(
			"GetWordOrderDraft() = %v, %v; want %v, nil",
			got,
			err,
			want,
		)
	}

	rdb.values[key] = "{malformed"
	got, err = store.GetWordOrderDraft(
		ctx,
		12,
		4,
	)
	if err != nil || got != nil {
		t.Fatalf(
			"GetWordOrderDraft(corrupt) = %v, %v; want nil, nil",
			got,
			err,
		)
	}
	if _, exists := rdb.values[key]; exists {
		t.Fatalf(
			"GetWordOrderDraft(corrupt) left %q",
			key,
		)
	}

	rdb.values[key] = "{malformed"
	rdb.delErr = errors.New("redis delete unavailable")
	got, err = store.GetWordOrderDraft(
		ctx,
		12,
		4,
	)
	if err != nil || got != nil {
		t.Fatalf(
			"GetWordOrderDraft(corrupt, delete error) = %v, %v; want nil, nil",
			got,
			err,
		)
	}
	if !reflect.DeepEqual(
		rdb.delCalls[len(rdb.delCalls)-1],
		[]string{key},
	) {
		t.Fatalf(
			"corrupt draft delete keys = %v, want [%s]",
			rdb.delCalls[len(rdb.delCalls)-1],
			key,
		)
	}
}

func TestMessageAndSessionMetadataContracts(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeInteractionsRedis()
	store := NewInteractions(rdb)
	ref := model.TelegramMessageRef{ChatID: -100123, MessageID: 45}
	messageKey := "handwriting:msg:7:9"

	if err := store.SaveHandwritingMessage(
		ctx,
		7,
		9,
		ref,
	); err != nil {
		t.Fatalf(
			"SaveHandwritingMessage() error = %v",
			err,
		)
	}
	assertInteractionSet(
		t,
		rdb,
		messageKey,
		"-100123:45",
		time.Hour,
	)
	gotRef, err := store.GetHandwritingMessage(
		ctx,
		7,
		9,
	)
	if err != nil || gotRef == nil || *gotRef != ref {
		t.Fatalf(
			"GetHandwritingMessage() = %+v, %v; want %+v, nil",
			gotRef,
			err,
			ref,
		)
	}
	missingRef, err := store.GetHandwritingMessage(
		ctx,
		7,
		10,
	)
	if err != nil || missingRef != nil {
		t.Fatalf(
			"GetHandwritingMessage(missing) = %+v, %v; want nil, nil",
			missingRef,
			err,
		)
	}
	rdb.values["handwriting:msg:7:11"] = "invalid"
	if _, err := store.GetHandwritingMessage(
		ctx,
		7,
		11,
	); !errors.Is(
		err,
		model.ErrInvalidTelegramMessageRef,
	) {
		t.Fatalf(
			"GetHandwritingMessage(invalid) error = %v, want invalid reference",
			err,
		)
	}

	fingerprintKey := "copylingo:miniapp:last_fingerprint:7"
	if err := store.SetMiniAppFingerprint(
		ctx,
		7,
		"ab12cd34",
	); err != nil {
		t.Fatalf(
			"SetMiniAppFingerprint() error = %v",
			err,
		)
	}
	assertInteractionSet(
		t,
		rdb,
		fingerprintKey,
		"ab12cd34",
		24*time.Hour,
	)
	if got, err := store.GetMiniAppFingerprint(
		ctx,
		7,
	); err != nil || got != "ab12cd34" {
		t.Fatalf(
			"GetMiniAppFingerprint() = %q, %v; want fingerprint, nil",
			got,
			err,
		)
	}

	getErr := errors.New("redis read unavailable")
	rdb.getErr = getErr
	if _, err := store.GetHandwritingMessage(
		ctx,
		7,
		9,
	); !errors.Is(
		err,
		getErr,
	) {
		t.Fatalf(
			"GetHandwritingMessage(redis error) = %v, want wrapped Redis error",
			err,
		)
	}
}
