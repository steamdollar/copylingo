package bot

import (
	"context"
	"encoding/json"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type mockBotAPI struct {
	sentMessages []tgbotapi.Chattable
	sendErr      error
	// returnVoiceFileID, when set, is echoed back as the file_id of a sent voice
	// message so file_id-caching paths can be exercised.
	returnVoiceFileID string
}

func (m *mockBotAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.sentMessages = append(
		m.sentMessages,
		c,
	)
	if m.sendErr != nil {
		return tgbotapi.Message{}, m.sendErr
	}
	if _, ok := c.(tgbotapi.VoiceConfig); ok && m.returnVoiceFileID != "" {
		return tgbotapi.Message{MessageID: 1001, Voice: &tgbotapi.Voice{FileID: m.returnVoiceFileID}}, nil
	}
	return tgbotapi.Message{MessageID: 1001}, nil
}

func (m *mockBotAPI) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	return &tgbotapi.APIResponse{}, nil
}

func (m *mockBotAPI) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return nil
}

func (m *mockBotAPI) StopReceivingUpdates() {}

type testInteractionStores struct {
	quiz         *testQuizSessionStore
	study        *testStudySessionStore
	pending      map[int64]model.PendingLLMInput
	active       map[int64]model.ActiveQuestionRef
	drafts       map[draftKey][]int
	messages     map[handwritingMessageKey]model.TelegramMessageRef
	fingerprints map[int]string
	starts       map[int]time.Time
}

type draftKey struct{ sessionID, questionID int }
type handwritingMessageKey struct{ sessionID, questionID int }

type testQuizSessionStore struct {
	states map[int]*model.QuizActiveSessionState
}
type testStudySessionStore struct {
	states map[int]*model.StudyActiveSessionState
}

func newTestInteractionStores() *testInteractionStores {
	return &testInteractionStores{
		quiz:         &testQuizSessionStore{states: map[int]*model.QuizActiveSessionState{}},
		study:        &testStudySessionStore{states: map[int]*model.StudyActiveSessionState{}},
		pending:      map[int64]model.PendingLLMInput{},
		active:       map[int64]model.ActiveQuestionRef{},
		drafts:       map[draftKey][]int{},
		messages:     map[handwritingMessageKey]model.TelegramMessageRef{},
		fingerprints: map[int]string{},
		starts:       map[int]time.Time{},
	}
}

func seedQuizState(
	stores *testInteractionStores,
	state *model.QuizActiveSessionState,
) {
	clone, err := cloneTestState(state)
	if err != nil {
		panic(err)
	}
	stores.quiz.states[state.Session.ID] = clone
}

func seedStudyState(
	stores *testInteractionStores,
	state *model.StudyActiveSessionState,
) {
	clone, err := cloneTestState(state)
	if err != nil {
		panic(err)
	}
	stores.study.states[state.Session.ID] = clone
}

func cloneTestState[T any](state *T) (*T, error) {
	if state == nil {
		return nil, nil
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	var clone T
	if err := json.Unmarshal(
		raw,
		&clone,
	); err != nil {
		return nil, err
	}
	return &clone, nil
}

// newTestSessionService wires SessionService over test fakes. When stores is
// non-nil, its Quiz/Study working-set stores back the service; deps left
// unset stay nil so an unexpected call fails loudly.
func newTestSessionService(
	stores *testInteractionStores,
	deps service.SessionDeps,
) *service.SessionService {
	if stores != nil {
		deps.Stores = service.SessionStores{Quiz: stores.quiz, Study: stores.study}
	}
	return service.NewSessionService(deps)
}

func (s *testInteractionStores) stateStores() StateStores {
	return StateStores{Input: s, Drafts: s, Messages: s, Recovery: s, Timing: s}
}

func (s *testQuizSessionStore) Load(
	_ context.Context,
	sessionID int,
) (*model.QuizActiveSessionState, error) {
	state, ok := s.states[sessionID]
	if !ok {
		return nil, model.ErrSessionStoreNotFound
	}
	return cloneTestState(state)
}

func (s *testQuizSessionStore) Save(
	_ context.Context,
	sessionID int,
	state *model.QuizActiveSessionState,
) error {
	clone, err := cloneTestState(state)
	if err != nil {
		return err
	}
	s.states[sessionID] = clone
	return nil
}

func (s *testQuizSessionStore) Delete(
	_ context.Context,
	sessionID int,
) error {
	delete(
		s.states,
		sessionID,
	)
	return nil
}

func (s *testStudySessionStore) Load(
	_ context.Context,
	sessionID int,
) (*model.StudyActiveSessionState, error) {
	state, ok := s.states[sessionID]
	if !ok {
		return nil, model.ErrSessionStoreNotFound
	}
	return cloneTestState(state)
}

func (s *testStudySessionStore) Save(
	_ context.Context,
	sessionID int,
	state *model.StudyActiveSessionState,
) error {
	clone, err := cloneTestState(state)
	if err != nil {
		return err
	}
	s.states[sessionID] = clone
	return nil
}

func (s *testStudySessionStore) Delete(
	_ context.Context,
	sessionID int,
) error {
	delete(
		s.states,
		sessionID,
	)
	return nil
}

func (s *testInteractionStores) SetLLMPending(
	_ context.Context,
	userID int64,
	input model.PendingLLMInput,
) error {
	s.pending[userID] = input
	return nil
}

func (s *testInteractionStores) TakeLLMPending(
	_ context.Context,
	userID int64,
) (model.PendingLLMInput, bool, error) {
	input, ok := s.pending[userID]
	delete(
		s.pending,
		userID,
	)
	return input, ok, nil
}

func (s *testInteractionStores) DeleteLLMPending(
	_ context.Context,
	userID int64,
) error {
	delete(
		s.pending,
		userID,
	)
	return nil
}

func (s *testInteractionStores) GetActiveQuestion(
	_ context.Context,
	chatID int64,
) (*model.ActiveQuestionRef, error) {
	question, ok := s.active[chatID]
	if !ok {
		return nil, nil
	}
	return &question, nil
}

func (s *testInteractionStores) SetActiveQuestion(
	_ context.Context,
	chatID int64,
	question model.ActiveQuestionRef,
) error {
	s.active[chatID] = question
	return nil
}

func (s *testInteractionStores) DeleteActiveQuestion(
	_ context.Context,
	chatID int64,
) error {
	delete(
		s.active,
		chatID,
	)
	return nil
}

func (s *testInteractionStores) ClearInput(
	_ context.Context,
	chatID int64,
	userID *int64,
) error {
	delete(
		s.active,
		chatID,
	)
	if userID != nil {
		delete(
			s.pending,
			*userID,
		)
	}
	return nil
}

func (s *testInteractionStores) GetWordOrderDraft(
	_ context.Context,
	sessionID,
	questionID int,
) ([]int, error) {
	selection, ok := s.drafts[draftKey{sessionID, questionID}]
	if !ok {
		return nil, nil
	}
	return append(
		[]int(nil),
		selection...,
	), nil
}

func (s *testInteractionStores) SetWordOrderDraft(
	_ context.Context,
	sessionID,
	questionID int,
	selection []int,
) error {
	s.drafts[draftKey{sessionID, questionID}] = append(
		[]int(nil),
		selection...,
	)
	return nil
}

func (s *testInteractionStores) DeleteWordOrderDraft(
	_ context.Context,
	sessionID,
	questionID int,
) error {
	delete(
		s.drafts,
		draftKey{sessionID, questionID},
	)
	return nil
}

func (s *testInteractionStores) SaveHandwritingMessage(
	_ context.Context,
	sessionID,
	questionID int,
	ref model.TelegramMessageRef,
) error {
	s.messages[handwritingMessageKey{sessionID, questionID}] = ref
	return nil
}

func (s *testInteractionStores) GetHandwritingMessage(
	_ context.Context,
	sessionID,
	questionID int,
) (*model.TelegramMessageRef, error) {
	ref, ok := s.messages[handwritingMessageKey{sessionID, questionID}]
	if !ok {
		return nil, nil
	}
	return &ref, nil
}

func (s *testInteractionStores) GetMiniAppFingerprint(
	_ context.Context,
	sessionID int,
) (string, error) {
	return s.fingerprints[sessionID], nil
}

func (s *testInteractionStores) SetMiniAppFingerprint(
	_ context.Context,
	sessionID int,
	fingerprint string,
) error {
	s.fingerprints[sessionID] = fingerprint
	return nil
}

func (s *testInteractionStores) RecordQuestionStart(
	_ context.Context,
	sessionID int,
	startedAt time.Time,
) error {
	s.starts[sessionID] = startedAt
	return nil
}

type mockSRS struct {
	service.SRSService
}

func (m *mockSRS) ScheduleAnswer(
	q *model.UserQuestionProgress,
	isCorrect bool,
) {
}
func (m *mockSRS) GetDueCount(
	ctx context.Context,
	userID int64,
	language,
	level string,
) (int, error) {
	return 0, nil
}

type mockLLM struct {
	gradeFn func(
		ctx context.Context,
		prompt,
		correctAnswer,
		userAnswer string,
	) (external.GradeResult, error)
	answerFn func(
		ctx context.Context,
		question string,
	) (string, error)
}

func (m *mockLLM) GradeAnswer(
	ctx context.Context,
	prompt,
	correctAnswer,
	userAnswer string,
) (external.GradeResult, error) {
	if m.gradeFn != nil {
		return m.gradeFn(
			ctx,
			prompt,
			correctAnswer,
			userAnswer,
		)
	}
	return external.GradeResult{IsCorrect: true}, nil
}

func (m *mockLLM) GradeHandwriting(
	ctx context.Context,
	prompt,
	correctAnswer string,
	image []byte,
) (external.GradeResult, error) {
	return external.GradeResult{}, nil
}
func (m *mockLLM) AnswerLearningQuestion(
	ctx context.Context,
	question string,
) (string, error) {
	if m.answerFn != nil {
		return m.answerFn(
			ctx,
			question,
		)
	}
	return "answer", nil
}
