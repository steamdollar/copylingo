package bot

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type mockBotAPI struct {
	sentMessages []tgbotapi.Chattable
	requests     []tgbotapi.Chattable
	sendErr      error
	// returnVoiceFileID is echoed back as a sent voice message's file_id to exercise file_id caching.
	returnVoiceFileID string
}

func (mock *mockBotAPI) Send(chattable tgbotapi.Chattable) (tgbotapi.Message, error) {
	mock.sentMessages = append(
		mock.sentMessages,
		chattable,
	)
	if mock.sendErr != nil {
		return tgbotapi.Message{}, mock.sendErr
	}
	if _, ok := chattable.(tgbotapi.VoiceConfig); ok && mock.returnVoiceFileID != "" {
		return tgbotapi.Message{MessageID: 1001, Voice: &tgbotapi.Voice{FileID: mock.returnVoiceFileID}}, nil
	}
	return tgbotapi.Message{MessageID: 1001}, nil
}

func (mock *mockBotAPI) Request(chattable tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	mock.requests = append(
		mock.requests,
		chattable,
	)
	return &tgbotapi.APIResponse{}, nil
}

func (mock *mockBotAPI) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return nil
}

func (mock *mockBotAPI) StopReceivingUpdates() {}

type testInteractionStores struct {
	quiz         *testQuizSessionStore
	study        *testStudySessionStore
	pending      map[int64]model.PendingLLMInput
	active       map[int64]model.ActiveQuestionReference
	drafts       map[draftKey][]int
	messages     map[handwritingMessageKey]model.TelegramMessageReference
	fingerprints map[int]string
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
		active:       map[int64]model.ActiveQuestionReference{},
		drafts:       map[draftKey][]int{},
		messages:     map[handwritingMessageKey]model.TelegramMessageReference{},
		fingerprints: map[int]string{},
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

// A DB without sessions: reloading a Quiz working set fails like the real repository. Any other call panics.
type missingQuizSessionRepository struct {
	service.QuizActiveSessionRepository
}

func (missingQuizSessionRepository) LoadQuestionSessionWithStateBySessionID(
	context.Context,
	int,
) (*model.QuizActiveSessionState, error) {
	return nil, sql.ErrNoRows
}

// Unset deps stay nil so an unexpected call fails loudly.
func newTestSessionService(
	stores *testInteractionStores,
	dependencies service.SessionDependencies,
) *service.SessionService {
	if stores != nil {
		dependencies.Stores = service.SessionStores{Quiz: stores.quiz, Study: stores.study}
	}
	if dependencies.QuizActiveSessionRepository == nil {
		dependencies.QuizActiveSessionRepository = missingQuizSessionRepository{}
	}
	return service.NewSessionService(dependencies)
}

func answerByText(
	t *testing.T,
	sessionFlow *SessionFlow,
	stores *testInteractionStores,
	chatID int64,
	from *tgbotapi.User,
	sessionID,
	questionIndex int,
	text string,
) {
	t.Helper()
	stores.active[chatID] = model.ActiveQuestionReference{SessionID: sessionID, QuestionIndex: questionIndex}
	if !sessionFlow.HandleTextInput(
		context.Background(),
		&tgbotapi.Message{Chat: &tgbotapi.Chat{ID: chatID}, From: from, Text: text},
	) {
		t.Fatal("text answer was not consumed")
	}
}

// Unset service deps stay nil so an unexpected call fails loudly.
func newTestSessionFlow(
	api *mockBotAPI,
	stores *testInteractionStores,
	dependencies SessionFlowDependencies,
) *SessionFlow {
	if api != nil {
		dependencies.Telegram = newTelegramClient(api)
	}
	if stores != nil {
		if dependencies.Input == nil {
			dependencies.Input = stores
		}
		if dependencies.Drafts == nil {
			dependencies.Drafts = stores
		}
		if dependencies.Messages == nil {
			dependencies.Messages = stores
		}
		if dependencies.Recovery == nil {
			dependencies.Recovery = stores
		}
	}
	return NewSessionFlow(dependencies)
}

func newTestStudyFlow(
	api *mockBotAPI,
	stores *testInteractionStores,
	dependencies StudyFlowDependencies,
) *StudyFlow {
	if api != nil {
		dependencies.Telegram = newTelegramClient(api)
	}
	if stores != nil && dependencies.Input == nil {
		dependencies.Input = stores
	}
	return NewStudyFlow(dependencies)
}

func newTestSettingsFlow(
	api *mockBotAPI,
	dependencies SettingsFlowDependencies,
) *SettingsFlow {
	if api != nil {
		dependencies.Telegram = newTelegramClient(api)
	}
	return NewSettingsFlow(dependencies)
}

func newTestLLMQuestionFlow(
	api *mockBotAPI,
	stores *testInteractionStores,
	dependencies LLMQuestionFlowDependencies,
) *LLMQuestionFlow {
	if api != nil {
		dependencies.Telegram = newTelegramClient(api)
	}
	if stores != nil && dependencies.Input == nil {
		dependencies.Input = stores
	}
	return NewLLMQuestionFlow(dependencies)
}

type testServices struct {
	Session            *service.SessionService
	User               *service.UserService
	MaterialPreference *service.MaterialPreferenceService
	Analyzer           *service.AnalyzerService
	LLMQuestion        *service.LLMQuestionService
	Audio              *service.AudioService
}

// A nil services wires no service dependency, a nil stores leaves every store unset.
func newTestBot(
	api *mockBotAPI,
	stores *testInteractionStores,
	services *testServices,
) *Bot {
	if services == nil {
		services = &testServices{}
	}
	studyFlow := newTestStudyFlow(
		api,
		stores,
		StudyFlowDependencies{
			Session:            services.Session,
			MaterialPreference: services.MaterialPreference,
		},
	)
	sessionDependencies := SessionFlowDependencies{
		Session:            services.Session,
		User:               services.User,
		MaterialPreference: services.MaterialPreference,
		Study:              studyFlow,
	}
	if services.Audio != nil {
		sessionDependencies.Audio = services.Audio
	}
	dependencies := BotDependencies{
		Telegram: newTelegramClient(api),
		User:     services.User,
		Session:  services.Session,
		Analyzer: services.Analyzer,
		SessionFlow: newTestSessionFlow(
			api,
			stores,
			sessionDependencies,
		),
		StudyFlow: studyFlow,
		SettingsFlow: newTestSettingsFlow(
			api,
			SettingsFlowDependencies{
				User:               services.User,
				MaterialPreference: services.MaterialPreference,
			},
		),
		LLMQuestionFlow: newTestLLMQuestionFlow(
			api,
			stores,
			LLMQuestionFlowDependencies{
				User:        services.User,
				LLMQuestion: services.LLMQuestion,
				Session:     services.Session,
			},
		),
	}
	// A nil *testInteractionStores in the interface would be a non-nil Input.
	if stores != nil {
		dependencies.Input = stores
	}
	return NewBot(dependencies)
}

func (store *testQuizSessionStore) Load(
	_ context.Context,
	sessionID int,
) (*model.QuizActiveSessionState, error) {
	state, ok := store.states[sessionID]
	if !ok {
		return nil, model.ErrSessionStoreNotFound
	}
	return cloneTestState(state)
}

func (store *testQuizSessionStore) Save(
	_ context.Context,
	sessionID int,
	state *model.QuizActiveSessionState,
) error {
	clone, err := cloneTestState(state)
	if err != nil {
		return err
	}
	store.states[sessionID] = clone
	return nil
}

func (store *testQuizSessionStore) Delete(
	_ context.Context,
	sessionID int,
) error {
	delete(
		store.states,
		sessionID,
	)
	return nil
}

func (store *testStudySessionStore) Load(
	_ context.Context,
	sessionID int,
) (*model.StudyActiveSessionState, error) {
	state, ok := store.states[sessionID]
	if !ok {
		return nil, model.ErrSessionStoreNotFound
	}
	return cloneTestState(state)
}

func (store *testStudySessionStore) Save(
	_ context.Context,
	sessionID int,
	state *model.StudyActiveSessionState,
) error {
	clone, err := cloneTestState(state)
	if err != nil {
		return err
	}
	store.states[sessionID] = clone
	return nil
}

func (store *testStudySessionStore) Delete(
	_ context.Context,
	sessionID int,
) error {
	delete(
		store.states,
		sessionID,
	)
	return nil
}

func (stores *testInteractionStores) SetLLMPending(
	_ context.Context,
	userID int64,
	input model.PendingLLMInput,
) error {
	stores.pending[userID] = input
	return nil
}

func (stores *testInteractionStores) TakeLLMPending(
	_ context.Context,
	userID int64,
) (model.PendingLLMInput, bool, error) {
	input, ok := stores.pending[userID]
	delete(
		stores.pending,
		userID,
	)
	return input, ok, nil
}

func (stores *testInteractionStores) DeleteLLMPending(
	_ context.Context,
	userID int64,
) error {
	delete(
		stores.pending,
		userID,
	)
	return nil
}

func (stores *testInteractionStores) GetActiveQuestion(
	_ context.Context,
	chatID int64,
) (*model.ActiveQuestionReference, error) {
	question, ok := stores.active[chatID]
	if !ok {
		return nil, nil
	}
	return &question, nil
}

func (stores *testInteractionStores) SetActiveQuestion(
	_ context.Context,
	chatID int64,
	question model.ActiveQuestionReference,
) error {
	stores.active[chatID] = question
	return nil
}

func (stores *testInteractionStores) DeleteActiveQuestion(
	_ context.Context,
	chatID int64,
) error {
	delete(
		stores.active,
		chatID,
	)
	return nil
}

func (stores *testInteractionStores) ClearInput(
	_ context.Context,
	chatID int64,
	userID *int64,
) error {
	delete(
		stores.active,
		chatID,
	)
	if userID != nil {
		delete(
			stores.pending,
			*userID,
		)
	}
	return nil
}

func (stores *testInteractionStores) GetWordOrderDraft(
	_ context.Context,
	sessionID,
	questionID int,
) ([]int, error) {
	selection, ok := stores.drafts[draftKey{sessionID, questionID}]
	if !ok {
		return nil, nil
	}
	return append(
		[]int(nil),
		selection...,
	), nil
}

func (stores *testInteractionStores) SetWordOrderDraft(
	_ context.Context,
	sessionID,
	questionID int,
	selection []int,
) error {
	stores.drafts[draftKey{sessionID, questionID}] = append(
		[]int(nil),
		selection...,
	)
	return nil
}

func (stores *testInteractionStores) DeleteWordOrderDraft(
	_ context.Context,
	sessionID,
	questionID int,
) error {
	delete(
		stores.drafts,
		draftKey{sessionID, questionID},
	)
	return nil
}

func (stores *testInteractionStores) SaveHandwritingMessage(
	_ context.Context,
	sessionID,
	questionID int,
	messageReference model.TelegramMessageReference,
) error {
	stores.messages[handwritingMessageKey{sessionID, questionID}] = messageReference
	return nil
}

func (stores *testInteractionStores) GetHandwritingMessage(
	_ context.Context,
	sessionID,
	questionID int,
) (*model.TelegramMessageReference, error) {
	messageReference, ok := stores.messages[handwritingMessageKey{sessionID, questionID}]
	if !ok {
		return nil, nil
	}
	return &messageReference, nil
}

func (stores *testInteractionStores) GetMiniAppFingerprint(
	_ context.Context,
	sessionID int,
) (string, error) {
	return stores.fingerprints[sessionID], nil
}

func (stores *testInteractionStores) SetMiniAppFingerprint(
	_ context.Context,
	sessionID int,
	fingerprint string,
) error {
	stores.fingerprints[sessionID] = fingerprint
	return nil
}

type mockLLM struct {
	answerFunc func(
		ctx context.Context,
		question string,
	) (string, error)
}

func (mock *mockLLM) GradeHandwriting(
	ctx context.Context,
	prompt,
	correctAnswer string,
	image []byte,
) (external.GradeResult, error) {
	return external.GradeResult{}, nil
}
func (mock *mockLLM) AnswerLearningQuestion(
	ctx context.Context,
	question string,
) (string, error) {
	if mock.answerFunc != nil {
		return mock.answerFunc(
			ctx,
			question,
		)
	}
	return "answer", nil
}
