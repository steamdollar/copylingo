package bot

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type botMaterialPreferenceRepository struct {
	items     map[[2]int64]model.MaterialPreference
	getCalls  int
	setCalls  int
	listCalls int
	listUser  int64
	listStart int
	listLimit int
}

func (repository *botMaterialPreferenceRepository) Get(
	_ context.Context,
	userID int64,
	materialID int,
) (*model.MaterialPreference, error) {
	repository.getCalls++
	item, ok := repository.items[[2]int64{userID, int64(materialID)}]
	if !ok {
		return nil, nil
	}
	return &item, nil
}

func (repository *botMaterialPreferenceRepository) Set(
	_ context.Context,
	userID int64,
	materialID int,
	mode model.MaterialReviewMode,
) error {
	repository.setCalls++
	key := [2]int64{userID, int64(materialID)}
	if mode == model.MaterialReviewNormal {
		delete(
			repository.items,
			key,
		)
		return nil
	}
	item := repository.items[key]
	item.UserID, item.MaterialID, item.ReviewMode = userID, materialID, mode
	repository.items[key] = item
	return nil
}

func (repository *botMaterialPreferenceRepository) List(
	_ context.Context,
	userID int64,
	offset,
	limit int,
) ([]model.MaterialPreference, error) {
	repository.listCalls++
	repository.listUser, repository.listStart, repository.listLimit = userID, offset, limit
	var items []model.MaterialPreference
	for _, item := range repository.items {
		if item.UserID == userID {
			items = append(
				items,
				item,
			)
		}
	}
	sort.Slice(
		items,
		func(
			i,
			j int,
		) bool {
			return items[i].MaterialID < items[j].MaterialID
		},
	)
	if offset >= len(items) {
		return nil, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], nil
}

func preferenceCallback(
	data string,
	userID int64,
) *tgbotapi.CallbackQuery {
	return &tgbotapi.CallbackQuery{
		Data: data,
		From: &tgbotapi.User{ID: userID},
		Message: &tgbotapi.Message{
			MessageID: 55,
			Chat:      &tgbotapi.Chat{ID: -999}, // Never use the chat as the preference owner.
		},
	}
}

func newPreferenceStudyBot(
	t *testing.T,
) (*StudyFlow, *botMaterialPreferenceRepository, *testInteractionStores, *botStudyActiveRepository) {
	t.Helper()
	activeRepository := &botStudyActiveRepository{
		session: &model.Session{ID: 77, UserID: 42, Mode: model.SessionModeStudy, Status: model.SessionInProgress},
		items: []model.StudySessionMaterial{
			studyItem(
				77,
				10,
				0,
				"<水 & 물>",
				vocabularyPayload(
					"みず",
					"水",
					"물",
					"noun",
				),
			),
			studyItem(
				77,
				11,
				1,
				"ひと",
				vocabularyPayload(
					"ひと",
					"人",
					"사람",
					"noun",
				),
			),
		},
	}
	stateStores := newTestInteractionStores()
	session := newTestSessionService(
		stateStores,
		service.SessionDependencies{StudyActiveSessionRepository: activeRepository},
	)
	if _, err := session.StudyProgress(
		context.Background(),
		77,
		42,
	); err != nil {
		t.Fatal(err)
	}
	repository := &botMaterialPreferenceRepository{items: make(map[[2]int64]model.MaterialPreference)}
	flow := newTestStudyFlow(
		&mockBotAPI{},
		nil,
		StudyFlowDependencies{
			Session:            session,
			MaterialPreference: service.NewMaterialPreferenceService(repository),
		},
	)
	return flow, repository, stateStores, activeRepository
}

func newPreferenceQuizBot(t *testing.T) (*SessionFlow, *botMaterialPreferenceRepository, *testInteractionStores) {
	t.Helper()
	linkedMaterialID := 10
	stateStores := newTestInteractionStores()
	storeActiveState(
		t,
		stateStores,
		77,
		&model.QuizActiveSessionState{
			Version: model.QuizActiveSessionStateVersion,
			Session: model.Session{ID: 77, UserID: 42, Mode: model.SessionModeQuiz, Status: model.SessionInProgress},
			Items: []model.QuizActiveSessionQuestion{
				{SessionQuestion: model.SessionQuestion{QuestionID: 101}, Question: model.Question{
					ID: 101, MaterialID: &linkedMaterialID, Type: model.QuestionMultipleChoice,
					Prompt: "Pick one", Options: []byte(`["A","B"]`),
				}},
				{SessionQuestion: model.SessionQuestion{QuestionID: 102}, Question: model.Question{
					ID: 102, Type: model.QuestionMultipleChoice, Prompt: "Unlinked", Options: []byte(`["A","B"]`),
				}},
			},
		},
	)
	repository := &botMaterialPreferenceRepository{items: make(map[[2]int64]model.MaterialPreference)}
	flow := newTestSessionFlow(
		&mockBotAPI{},
		stateStores,
		SessionFlowDependencies{
			Session: newTestSessionService(
				stateStores,
				service.SessionDependencies{},
			),
			MaterialPreference: service.NewMaterialPreferenceService(repository),
			PublicBaseURL:      "https://example.com",
		},
	)
	return flow, repository, stateStores
}

func TestQuizMaterialPreferenceUsesSharedMenuAndPreservesSession(t *testing.T) {
	flow, repository, stateStores := newPreferenceQuizBot(t)
	before, err := stateStores.quiz.Load(
		context.Background(),
		77,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, callback := range []struct {
		data   string
		userID int64
	}{
		{"q:77:policy:101", 43}, // Another user's session.
		{"q:77:policy:999", 42}, // Not in this session.
		{"q:77:policy:102", 42}, // No linked material.
		{"q:77:policy:bad", 42},
		{"q:77:policy:101:bogus", 42},
		{"q:77:exclude:101:excluded", 42},
	} {
		flow.HandleAnswerCallback(
			context.Background(),
			preferenceCallback(
				callback.data,
				callback.userID,
			),
		)
	}
	if repository.getCalls != 0 || repository.setCalls != 0 {
		t.Fatalf(
			"unauthorized or invalid callback reached preferences: %+v",
			repository,
		)
	}
	// Buttons sent before this change open the menu instead of excluding directly.
	flow.HandleAnswerCallback(
		context.Background(),
		preferenceCallback(
			"q:77:exclude:101",
			42,
		),
	)
	if repository.getCalls != 1 || repository.setCalls != 0 {
		t.Fatalf(
			"opening the menu changed the preference: %+v",
			repository,
		)
	}
	messages := flow.telegram.api.(*mockBotAPI).sentMessages
	menu, ok := messages[len(messages)-1].(tgbotapi.MessageConfig)
	if !ok || !strings.Contains(
		menu.Text,
		"현재: 일반 학습",
	) {
		t.Fatalf(
			"Quiz preference menu was not shown: %+v",
			messages[len(messages)-1],
		)
	}
	keyboard, ok := menu.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf(
			"Quiz preference choices were not shown: %+v",
			menu.ReplyMarkup,
		)
	}
	// Same three choices as the Study card, then a close row instead of
	// Study's back-to-card row.
	assertMaterialPreferenceMenu(
		t,
		keyboard,
		model.MaterialReviewNormal,
		func(mode model.MaterialReviewMode) string {
			return fmt.Sprintf(
				formatQuestionPolicySet,
				77,
				101,
				mode,
			)
		},
		"q:77:policy:101:close",
	)
	for _, mode := range []model.MaterialReviewMode{model.MaterialReviewMaintenance, model.MaterialReviewExcluded, model.MaterialReviewNormal} {
		flow.HandleAnswerCallback(
			context.Background(),
			preferenceCallback(
				fmt.Sprintf(
					formatQuestionPolicySet,
					77,
					101,
					mode,
				),
				42,
			),
		)
		edit := lastEditMessage(
			t,
			flow.telegram.api.(*mockBotAPI),
		)
		if edit.MessageID != 55 || !strings.Contains(
			edit.Text,
			"현재: "+materialReviewModeLabel(mode),
		) || edit.ReplyMarkup == nil {
			t.Fatalf(
				"Quiz selection did not re-render the menu for %s: %+v",
				mode,
				edit,
			)
		}
		assertMaterialPreferenceMenu(
			t,
			*edit.ReplyMarkup,
			mode,
			func(choice model.MaterialReviewMode) string {
				return fmt.Sprintf(
					formatQuestionPolicySet,
					77,
					101,
					choice,
				)
			},
			"q:77:policy:101:close",
		)
	}
	// Choosing normal last removes the stored row; no row means normal.
	if _, stored := repository.items[[2]int64{42, 10}]; repository.setCalls != 3 || stored {
		t.Fatalf(
			"Quiz preference choices did not update linked material: %+v",
			repository,
		)
	}
	after, err := stateStores.quiz.Load(
		context.Background(),
		77,
	)
	if err != nil || !reflect.DeepEqual(
		after,
		before,
	) {
		t.Fatal("Quiz preference callbacks changed the current session")
	}
}

func TestQuizMaterialPreferenceCloseDeletesOnlyMenuMessage(t *testing.T) {
	flow, repository, _ := newPreferenceQuizBot(t)
	api := flow.telegram.api.(*mockBotAPI)
	flow.HandleAnswerCallback(
		context.Background(),
		preferenceCallback(
			"q:77:policy:101:close",
			42,
		),
	)
	if repository.getCalls != 0 || repository.setCalls != 0 || len(api.sentMessages) != 0 {
		t.Fatalf(
			"close touched preferences or sent a message: repo=%+v sent=%+v",
			repository,
			api.sentMessages,
		)
	}
	if len(api.requests) != 1 {
		t.Fatalf(
			"close made %d requests, want one deleteMessage",
			len(api.requests),
		)
	}
	deleted, ok := api.requests[0].(tgbotapi.DeleteMessageConfig)
	if !ok || deleted.ChatID != -999 || deleted.MessageID != 55 {
		t.Fatalf(
			"close did not delete the menu message: %+v",
			api.requests[0],
		)
	}
}

func assertMaterialPreferenceMenu(
	t *testing.T,
	keyboard tgbotapi.InlineKeyboardMarkup,
	selected model.MaterialReviewMode,
	choiceCallbackData func(model.MaterialReviewMode) string,
	navigationCallbackData string,
) {
	t.Helper()
	modes := []model.MaterialReviewMode{
		model.MaterialReviewNormal,
		model.MaterialReviewMaintenance,
		model.MaterialReviewExcluded,
	}
	if len(keyboard.InlineKeyboard) != len(modes)+1 {
		t.Fatalf(
			"menu rows = %d, want %d: %+v",
			len(keyboard.InlineKeyboard),
			len(modes)+1,
			keyboard,
		)
	}
	for i, mode := range modes {
		button := keyboard.InlineKeyboard[i][0]
		label := materialReviewModeLabel(mode)
		if mode == selected {
			label = "✓ " + label
		}
		if button.Text != label || button.CallbackData == nil || *button.CallbackData != choiceCallbackData(mode) {
			t.Fatalf(
				"menu row %d = %q/%v, want %q/%q",
				i,
				button.Text,
				button.CallbackData,
				label,
				choiceCallbackData(mode),
			)
		}
	}
	navigation := keyboard.InlineKeyboard[len(modes)][0]
	if navigation.CallbackData == nil || *navigation.CallbackData != navigationCallbackData {
		t.Fatalf(
			"menu navigation = %+v, want %q",
			navigation,
			navigationCallbackData,
		)
	}
}

func TestQuizMaterialPreferenceButtonFollowsLinkedQuestions(t *testing.T) {
	flow, _, _ := newPreferenceQuizBot(t)
	flow.showQuestion(
		context.Background(),
		42,
		nil,
		77,
		0,
	)
	api := flow.telegram.api.(*mockBotAPI)
	message, ok := api.sentMessages[len(api.sentMessages)-1].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf(
			"quiz question was not sent: %T",
			api.sentMessages[len(api.sentMessages)-1],
		)
	}
	keyboard, ok := message.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	if !ok || !hasPreferenceCallback(
		&keyboard,
		"q:77:policy:101",
	) {
		t.Fatalf(
			"linked quiz question has no preference button: %+v",
			message.ReplyMarkup,
		)
	}
	flow.showQuestion(
		context.Background(),
		42,
		nil,
		77,
		1,
	)
	message = api.sentMessages[len(api.sentMessages)-1].(tgbotapi.MessageConfig)
	keyboard, ok = message.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	if !ok || hasPreferenceCallback(
		&keyboard,
		"q:77:policy:102",
	) {
		t.Fatalf(
			"unlinked quiz question has a preference button: %+v",
			message.ReplyMarkup,
		)
	}
	linkedMaterialID := 10
	api.sentMessages = nil
	_, _, done := flow.renderByType(
		context.Background(),
		42,
		nil,
		77,
		0,
		2,
		model.Question{
			ID: 103, MaterialID: &linkedMaterialID, Type: model.QuestionKanaHandwriting,
			Prompt: "Write kana", CorrectAnswer: "あ",
		},
		false,
	)
	if !done || len(api.sentMessages) != 1 {
		t.Fatal("linked handwriting question was not sent")
	}
	handwritingMessage := api.sentMessages[0].(tgbotapi.MessageConfig)
	handwritingKeyboard, ok := handwritingMessage.ReplyMarkup.(webAppKeyboardMarkup)
	if !ok || len(handwritingKeyboard.InlineKeyboard) != 3 ||
		handwritingKeyboard.InlineKeyboard[2][0].CallbackData == nil ||
		*handwritingKeyboard.InlineKeyboard[2][0].CallbackData != "q:77:policy:103" {
		t.Fatalf(
			"linked handwriting question has no preference button: %+v",
			handwritingMessage.ReplyMarkup,
		)
	}
}

func TestStudyMaterialPreferenceDoesNotChangeSessionProgress(t *testing.T) {
	flow, repository, stateStores, activeRepository := newPreferenceStudyBot(t)
	before, err := stateStores.study.Load(
		context.Background(),
		77,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{
		"study:77:policy:10",
		"study:77:policy:10:maintenance",
		"study:77:policy:10:excluded",
		"study:77:policy:10:normal",
		"study:77:card:10",
	} {
		flow.HandleCallback(
			context.Background(),
			preferenceCallback(
				data,
				42,
			),
		)
		got, err := stateStores.study.Load(
			context.Background(),
			77,
		)
		if err != nil || !reflect.DeepEqual(
			got,
			before,
		) {
			t.Fatalf(
				"%s changed current study session: %+v",
				data,
				got,
			)
		}
		if activeRepository.flushedState != nil {
			t.Fatalf(
				"%s flushed Study progress",
				data,
			)
		}
	}
	if repository.getCalls != 1 || repository.setCalls != 3 || len(repository.items) != 0 {
		t.Fatalf(
			"unexpected preference operations: %+v",
			repository,
		)
	}
	edit := lastEditMessage(
		t,
		flow.telegram.api.(*mockBotAPI),
	)
	if !strings.Contains(
		edit.Text,
		"1/2",
	) || !hasPreferenceCallback(
		edit.ReplyMarkup,
		"study:77:policy:10",
	) {
		t.Fatalf(
			"did not return to same study card: %+v",
			edit,
		)
	}
}

func TestStudyMaterialPreferenceRequiresOwnerAndMembership(t *testing.T) {
	for _, testCase := range []struct {
		name string
		data string
		user int64
	}{
		{"other owner read", "study:77:policy:10", 43},
		{"other owner write", "study:77:policy:10:excluded", 43},
		{"nonmember read", "study:77:policy:999", 42},
		{"nonmember write", "study:77:policy:999:excluded", 42},
		{"forged session", "study:999:policy:10:excluded", 42},
	} {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				flow, repository, _, _ := newPreferenceStudyBot(t)
				flow.HandleCallback(
					context.Background(),
					preferenceCallback(
						testCase.data,
						testCase.user,
					),
				)
				if repository.getCalls != 0 || repository.setCalls != 0 {
					t.Fatalf(
						"unauthorized access reached preference service: %+v",
						repository,
					)
				}
			},
		)
	}
}

func TestStudyMaterialPreferenceEscapesHTMLAndExplainsMaintenance(t *testing.T) {
	flow, _, _, _ := newPreferenceStudyBot(t)
	flow.HandleCallback(
		context.Background(),
		preferenceCallback(
			"study:77:policy:10:maintenance",
			42,
		),
	)
	edit := lastEditMessage(
		t,
		flow.telegram.api.(*mockBotAPI),
	)
	for _, want := range []string{"&lt;水 &amp; 물&gt;", "30일", "60→120→최대 180일", "오답이면 일반 학습", "새로 만드는 세션부터 적용"} {
		if !strings.Contains(
			edit.Text,
			want,
		) {
			t.Errorf(
				"preference menu missing %q: %s",
				want,
				edit.Text,
			)
		}
	}
	if edit.ReplyMarkup == nil {
		t.Fatal("Study preference menu lost its keyboard")
	}
	assertMaterialPreferenceMenu(
		t,
		*edit.ReplyMarkup,
		model.MaterialReviewMaintenance,
		func(mode model.MaterialReviewMode) string {
			return fmt.Sprintf(
				formatStudyPolicySet,
				77,
				10,
				mode,
			)
		},
		"study:77:card:10",
	)
}

func TestMaterialPreferencesListPaginationAndUserScopedRestore(t *testing.T) {
	repository := &botMaterialPreferenceRepository{items: make(map[[2]int64]model.MaterialPreference)}
	for materialID := 1; materialID <= 9; materialID++ {
		repository.items[[2]int64{42, int64(materialID)}] = model.MaterialPreference{
			UserID:     42,
			MaterialID: materialID,
			MaterialTitle: fmt.Sprintf(
				"<item %d &>",
				materialID,
			),
			ReviewMode: model.MaterialReviewMaintenance,
		}
	}
	otherKey := [2]int64{43, 9}
	repository.items[otherKey] = model.MaterialPreference{
		UserID:        43,
		MaterialID:    9,
		MaterialTitle: "other-user-secret",
		ReviewMode:    model.MaterialReviewExcluded,
	}
	api := &mockBotAPI{}
	flow := newTestSettingsFlow(
		api,
		SettingsFlowDependencies{MaterialPreference: service.NewMaterialPreferenceService(repository)},
	)
	flow.handleSettingsCallback(
		context.Background(),
		preferenceCallback(
			"settings:materials:0",
			42,
		),
	)
	edit := lastEditMessage(
		t,
		api,
	)
	if repository.listCalls != 1 || repository.listUser != 42 || repository.listStart != 0 ||
		repository.listLimit != 9 ||
		repository.getCalls != 0 {
		t.Fatalf(
			"expected one user-scoped batch query: %+v",
			repository,
		)
	}
	if !strings.Contains(
		edit.Text,
		"&lt;item 1 &amp;&gt;",
	) || strings.Contains(
		edit.Text,
		"other-user-secret",
	) ||
		strings.Contains(
			edit.Text,
			"item 9",
		) {
		t.Fatalf(
			"incorrect first page: %s",
			edit.Text,
		)
	}
	if !hasPreferenceCallback(
		edit.ReplyMarkup,
		"settings:materials:1",
	) ||
		hasPreferenceCallback(
			edit.ReplyMarkup,
			"settings:materials:-1",
		) {
		t.Fatal("incorrect first page navigation")
	}
	flow.handleSettingsCallback(
		context.Background(),
		preferenceCallback(
			"settings:materials:1",
			42,
		),
	)
	edit = lastEditMessage(
		t,
		api,
	)
	if repository.listStart != 8 || !strings.Contains(
		edit.Text,
		"item 9",
	) ||
		!hasPreferenceCallback(
			edit.ReplyMarkup,
			"settings:materials:0",
		) ||
		hasPreferenceCallback(
			edit.ReplyMarkup,
			"settings:materials:2",
		) {
		t.Fatalf(
			"incorrect second page: %+v",
			edit,
		)
	}
	for i := 0; i < 2; i++ {
		flow.handleSettingsCallback(
			context.Background(),
			preferenceCallback(
				"settings:restore:9:1",
				42,
			),
		)
	}
	if _, ok := repository.items[[2]int64{42, 9}]; ok {
		t.Fatal("current user's preference was not restored")
	}
	if repository.items[otherKey].ReviewMode != model.MaterialReviewExcluded {
		t.Fatal("restore changed another user's preference")
	}
	edit = lastEditMessage(
		t,
		api,
	)
	if !strings.Contains(
		edit.Text,
		"이 페이지에 설정한 항목이 없습니다",
	) ||
		!hasPreferenceCallback(
			edit.ReplyMarkup,
			"settings:materials:0",
		) ||
		!hasPreferenceCallback(
			edit.ReplyMarkup,
			callbackSettingsView,
		) {
		t.Fatalf(
			"empty last page lost navigation: %+v",
			edit,
		)
	}
}

func TestMaterialPreferenceCallbacksRejectMalformedInput(t *testing.T) {
	flow, repository, _, _ := newPreferenceStudyBot(t)
	for _, data := range []string{
		"study:77:policy", "study:77:policy:0", "study:77:policy:-1", "study:77:policy:10:bogus",
		"study:77:policy:10:normal:extra", "study:77:policy:10:", "study:0:policy:10:normal",
		"study:-1:policy:10:normal", "study:77:policy:+10:normal", "study:77:card:10:normal",
		"study:77:policy:99999999999999999999:normal", "study:77:policy:10:" + strings.Repeat(
			"x",
			65,
		),
	} {
		flow.HandleCallback(
			context.Background(),
			preferenceCallback(
				data,
				42,
			),
		)
	}
	settingsFlow := newTestSettingsFlow(
		nil,
		SettingsFlowDependencies{
			Telegram:           flow.telegram,
			MaterialPreference: service.NewMaterialPreferenceService(repository),
		},
	)
	for _, data := range []string{
		"settings:materials", "settings:restore",
		"settings:materials:-1", "settings:materials:x", "settings:materials:+1", "settings:materials:0:extra",
		"settings:restore:0:0", "settings:restore:-1:0", "settings:restore:1:-1", "settings:restore:1:0:extra",
		"settings:restore:1", "settings:restore:1:", "settings:materials:9999999999999999999",
		fmt.Sprintf(
			"settings:restore:10:%d",
			int(^uint(0)>>1),
		),
	} {
		settingsFlow.handleSettingsCallback(
			context.Background(),
			preferenceCallback(
				data,
				42,
			),
		)
	}
	if repository.getCalls != 0 || repository.setCalls != 0 || repository.listCalls != 0 {
		t.Fatalf(
			"malformed callback reached preference repository: %+v",
			repository,
		)
	}
}

func TestMaterialPreferenceKeyboardAvailabilityAndCallbackLength(t *testing.T) {
	keyboard := buildSettingsKeyboard(&model.User{})
	if !hasPreferenceCallback(
		&keyboard,
		"settings:materials:0",
	) ||
		!hasPreferenceCallback(
			&keyboard,
			callbackMenuMain,
		) {
		t.Fatal("preference menu or main navigation missing")
	}
	maxID := int(^uint(0) >> 1)
	_, quizKeyboard := quizMaterialPreferenceMenu(
		maxID,
		maxID,
		model.MaterialReviewMaintenance,
	)
	_, studyKeyboard := studyMaterialPreferenceMenu(
		"title",
		maxID,
		maxID,
		model.MaterialReviewMaintenance,
	)
	keyboards := []tgbotapi.InlineKeyboardMarkup{quizKeyboard, studyKeyboard}
	_, listKeyboard := materialPreferencesView(
		[]model.MaterialPreference{{MaterialID: maxID}},
		1,
		true,
	)
	keyboards = append(
		keyboards,
		listKeyboard,
	)
	for _, keyboard := range keyboards {
		for _, row := range keyboard.InlineKeyboard {
			for _, button := range row {
				if len(*button.CallbackData) > 64 {
					t.Errorf(
						"callback exceeds Telegram limit: %q",
						*button.CallbackData,
					)
				}
			}
		}
	}
}

func hasPreferenceCallback(
	keyboard *tgbotapi.InlineKeyboardMarkup,
	data string,
) bool {
	if keyboard == nil {
		return false
	}
	for _, row := range keyboard.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData != nil && *button.CallbackData == data {
				return true
			}
		}
	}
	return false
}
