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

func (fake *fakeInteractionsRedis) Get(
	_ context.Context,
	key string,
) *redis.StringCmd {
	fake.getKeys = append(
		fake.getKeys,
		key,
	)
	if fake.getErr != nil {
		return redis.NewStringResult(
			"",
			fake.getErr,
		)
	}
	value, ok := fake.values[key]
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

func (fake *fakeInteractionsRedis) GetDel(
	_ context.Context,
	key string,
) *redis.StringCmd {
	fake.getDelKeys = append(
		fake.getDelKeys,
		key,
	)
	if fake.getDelErr != nil {
		return redis.NewStringResult(
			"",
			fake.getDelErr,
		)
	}
	value, ok := fake.values[key]
	if !ok {
		return redis.NewStringResult(
			"",
			redis.Nil,
		)
	}
	delete(
		fake.values,
		key,
	)
	delete(
		fake.expiration,
		key,
	)
	return redis.NewStringResult(
		value,
		nil,
	)
}

func (fake *fakeInteractionsRedis) Set(
	_ context.Context,
	key string,
	value any,
	expiration time.Duration,
) *redis.StatusCmd {
	fake.setKeys = append(
		fake.setKeys,
		key,
	)
	if fake.setErr != nil {
		return redis.NewStatusResult(
			"",
			fake.setErr,
		)
	}
	switch value := value.(type) {
	case []byte:
		fake.values[key] = string(value)
	default:
		fake.values[key] = fmt.Sprint(value)
	}
	fake.expiration[key] = expiration
	return redis.NewStatusResult(
		"OK",
		nil,
	)
}

func (fake *fakeInteractionsRedis) Del(
	_ context.Context,
	keys ...string,
) *redis.IntCmd {
	fake.delCalls = append(
		fake.delCalls,
		append(
			[]string(nil),
			keys...,
		),
	)
	if fake.delErr != nil {
		return redis.NewIntResult(
			0,
			fake.delErr,
		)
	}
	var deleted int64
	for _, key := range keys {
		if _, ok := fake.values[key]; ok {
			delete(
				fake.values,
				key,
			)
			delete(
				fake.expiration,
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
	redisClient *fakeInteractionsRedis,
	key,
	value string,
	ttl time.Duration,
) {
	t.Helper()
	if got := redisClient.values[key]; got != value {
		t.Fatalf(
			"Redis value for %q = %q, want %q",
			key,
			got,
			value,
		)
	}
	if got := redisClient.expiration[key]; got != ttl {
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
	redisClient := newFakeInteractionsRedis()
	store := NewInteractions(redisClient)
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

	for _, testCase := range cases {
		key := fmt.Sprintf(
			"user:%d:llm_pending",
			testCase.userID,
		)
		if err := store.SetLLMPending(
			ctx,
			testCase.userID,
			testCase.input,
		); err != nil {
			t.Fatalf(
				"SetLLMPending(%d) error = %v",
				testCase.userID,
				err,
			)
		}
		assertInteractionSet(
			t,
			redisClient,
			key,
			testCase.value,
			10*time.Minute,
		)

		got, found, err := store.TakeLLMPending(
			ctx,
			testCase.userID,
		)
		if err != nil || !found || got != testCase.input {
			t.Fatalf(
				"TakeLLMPending(%d) = %+v, %t, %v; want %+v, true, nil",
				testCase.userID,
				got,
				found,
				err,
				testCase.input,
			)
		}
		if _, found, err := store.TakeLLMPending(
			ctx,
			testCase.userID,
		); err != nil || found {
			t.Fatalf(
				"second TakeLLMPending(%d) = found %t, error %v; want missing",
				testCase.userID,
				found,
				err,
			)
		}
	}

	malformedKey := "user:13:llm_pending"
	redisClient.values[malformedKey] = "q:bad:3"
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
		redisClient.getDelKeys,
		wantGetDelKeys,
	) {
		t.Fatalf(
			"pending GetDel keys = %v, want %v",
			redisClient.getDelKeys,
			wantGetDelKeys,
		)
	}
}

func TestActiveQuestionAndClearInputContracts(t *testing.T) {
	ctx := context.Background()
	redisClient := newFakeInteractionsRedis()
	store := NewInteractions(redisClient)
	chatID, userID := int64(123), int64(456)
	activeKey, pendingKey := "user:123:active_question", "user:456:llm_pending"
	question := model.ActiveQuestionReference{SessionID: 51, QuestionIndex: 2}

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
		redisClient,
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
	if len(redisClient.getKeys) != 1 || redisClient.getKeys[0] != activeKey || len(redisClient.getDelKeys) != 0 {
		t.Fatalf(
			"active question reads: GET %v, GETDEL %v; want GET only",
			redisClient.getKeys,
			redisClient.getDelKeys,
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
		redisClient.delCalls[len(redisClient.delCalls)-1],
		[]string{activeKey},
	) {
		t.Fatalf(
			"DeleteActiveQuestion() keys = %v, want [%s]",
			redisClient.delCalls[len(redisClient.delCalls)-1],
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
		redisClient.delCalls[len(redisClient.delCalls)-1],
		[]string{activeKey, pendingKey},
	) {
		t.Fatalf(
			"ClearInput(with user) keys = %v, want active question and pending keys",
			redisClient.delCalls[len(redisClient.delCalls)-1],
		)
	}
	if _, exists := redisClient.values[activeKey]; exists {
		t.Fatalf(
			"ClearInput(with user) left %q",
			activeKey,
		)
	}
	if _, exists := redisClient.values[pendingKey]; exists {
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
		redisClient.delCalls[len(redisClient.delCalls)-1],
		[]string{activeKey},
	) {
		t.Fatalf(
			"ClearInput(without user) keys = %v, want only active question key",
			redisClient.delCalls[len(redisClient.delCalls)-1],
		)
	}
	if _, ok := redisClient.values[pendingKey]; !ok {
		t.Fatal("ClearInput(without user) deleted the user pending key")
	}
}

func TestWordOrderDraftFormatAndCorruptCleanup(t *testing.T) {
	ctx := context.Background()
	redisClient := newFakeInteractionsRedis()
	store := NewInteractions(redisClient)
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
		redisClient,
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

	redisClient.values[key] = "{malformed"
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
	if _, exists := redisClient.values[key]; exists {
		t.Fatalf(
			"GetWordOrderDraft(corrupt) left %q",
			key,
		)
	}

	redisClient.values[key] = "{malformed"
	redisClient.delErr = errors.New("redis delete unavailable")
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
		redisClient.delCalls[len(redisClient.delCalls)-1],
		[]string{key},
	) {
		t.Fatalf(
			"corrupt draft delete keys = %v, want [%s]",
			redisClient.delCalls[len(redisClient.delCalls)-1],
			key,
		)
	}
}

func TestMessageAndSessionMetadataContracts(t *testing.T) {
	ctx := context.Background()
	redisClient := newFakeInteractionsRedis()
	store := NewInteractions(redisClient)
	messageReference := model.TelegramMessageReference{ChatID: -100123, MessageID: 45}
	messageKey := "handwriting:msg:7:9"

	if err := store.SaveHandwritingMessage(
		ctx,
		7,
		9,
		messageReference,
	); err != nil {
		t.Fatalf(
			"SaveHandwritingMessage() error = %v",
			err,
		)
	}
	assertInteractionSet(
		t,
		redisClient,
		messageKey,
		"-100123:45",
		time.Hour,
	)
	gotReference, err := store.GetHandwritingMessage(
		ctx,
		7,
		9,
	)
	if err != nil || gotReference == nil || *gotReference != messageReference {
		t.Fatalf(
			"GetHandwritingMessage() = %+v, %v; want %+v, nil",
			gotReference,
			err,
			messageReference,
		)
	}
	missingReference, err := store.GetHandwritingMessage(
		ctx,
		7,
		10,
	)
	if err != nil || missingReference != nil {
		t.Fatalf(
			"GetHandwritingMessage(missing) = %+v, %v; want nil, nil",
			missingReference,
			err,
		)
	}
	redisClient.values["handwriting:msg:7:11"] = "invalid"
	if _, err := store.GetHandwritingMessage(
		ctx,
		7,
		11,
	); !errors.Is(
		err,
		model.ErrInvalidTelegramMessageReference,
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
		redisClient,
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
	redisClient.getErr = getErr
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
