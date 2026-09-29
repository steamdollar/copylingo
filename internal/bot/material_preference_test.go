package bot

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type botMaterialPreferenceRepo struct {
	items     map[[2]int64]model.MaterialPreference
	getCalls  int
	setCalls  int
	listCalls int
	listUser  int64
	listStart int
	listLimit int
}

func (r *botMaterialPreferenceRepo) Get(
	_ context.Context,
	userID int64,
	materialID int,
) (*model.MaterialPreference, error) {
	r.getCalls++
	item, ok := r.items[[2]int64{userID, int64(materialID)}]
	if !ok {
		return nil, nil
	}
	return &item, nil
}

func (r *botMaterialPreferenceRepo) Set(
	_ context.Context,
	userID int64,
	materialID int,
	mode model.MaterialReviewMode,
) error {
	r.setCalls++
	key := [2]int64{userID, int64(materialID)}
	if mode == model.MaterialReviewNormal {
		delete(
			r.items,
			key,
		)
		return nil
	}
	item := r.items[key]
	item.UserID, item.MaterialID, item.ReviewMode = userID, materialID, mode
	r.items[key] = item
	return nil
}

func (r *botMaterialPreferenceRepo) List(
	_ context.Context,
	userID int64,
	offset,
	limit int,
) ([]model.MaterialPreference, error) {
	r.listCalls++
	r.listUser, r.listStart, r.listLimit = userID, offset, limit
	var items []model.MaterialPreference
	for _, item := range r.items {
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
) (*Bot, *botMaterialPreferenceRepo, *testInteractionStores, *botStudyActiveRepo) {
	t.Helper()
	activeRepo := &botStudyActiveRepo{
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
	active := service.NewStudyActiveSessionService(
		activeRepo,
		nil,
		stateStores.study,
	)
	if _, err := active.LoadOwnedStudySessionState(
		context.Background(),
		77,
		42,
	); err != nil {
		t.Fatal(err)
	}
	repo := &botMaterialPreferenceRepo{items: make(map[[2]int64]model.MaterialPreference)}
	b := &Bot{telegram: newTelegramClient(&mockBotAPI{}), services: &service.Services{
		StudyActiveSession: active,
		MaterialPreference: service.NewMaterialPreferenceService(repo),
	}}
	return b, repo, stateStores, activeRepo
}

func newPreferenceQuizBot(t *testing.T) (*Bot, *botMaterialPreferenceRepo, *testInteractionStores) {
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
	repo := &botMaterialPreferenceRepo{items: make(map[[2]int64]model.MaterialPreference)}
	bot := &Bot{
		telegram: newTelegramClient(&mockBotAPI{}),
		input:    stateStores,
		drafts:   stateStores,
		messages: stateStores,
		recovery: stateStores,
		timing:   stateStores,
		cfg:      &config.Config{Server: config.ServerConfig{PublicBaseURL: "https://example.com"}},
		services: &service.Services{
			QuizActiveSession: service.NewQuizActiveSessionService(
				nil,
				stateStores.quiz,
				nil,
			),
			MaterialPreference: service.NewMaterialPreferenceService(repo),
		},
	}
	return bot, repo, stateStores
}

func TestQuizMaterialPreferenceRequiresChoiceAndPreservesSession(t *testing.T) {
	bot, repo, stateStores := newPreferenceQuizBot(t)
	flow := NewSessionFlow(bot)
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
		{"q:77:policy:101:normal", 42}, // Quiz offers only maintenance or exclusion.
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
	if repo.getCalls != 0 || repo.setCalls != 0 {
		t.Fatalf(
			"unauthorized or invalid callback reached preferences: %+v",
			repo,
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
	if repo.getCalls != 1 || repo.setCalls != 0 {
		t.Fatalf(
			"opening the menu changed the preference: %+v",
			repo,
		)
	}
	messages := bot.telegram.api.(*mockBotAPI).sentMessages
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
	for _, mode := range []model.MaterialReviewMode{model.MaterialReviewMaintenance, model.MaterialReviewExcluded} {
		if !hasPreferenceCallback(
			&keyboard,
			fmt.Sprintf(
				formatQuestionPolicySet,
				77,
				101,
				mode,
			),
		) {
			t.Fatalf(
				"Quiz preference menu lacks %s",
				mode,
			)
		}
	}
	if len(keyboard.InlineKeyboard) != 2 || hasPreferenceCallback(
		&keyboard,
		"q:77:policy:101:normal",
	) ||
		keyboard.InlineKeyboard[0][0].Text != "유지 복습" || keyboard.InlineKeyboard[1][0].Text != "학습에서 제외" {
		t.Fatalf(
			"Quiz menu must show exactly two settings: %+v",
			keyboard,
		)
	}
	for _, mode := range []model.MaterialReviewMode{model.MaterialReviewMaintenance, model.MaterialReviewExcluded} {
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
			bot.telegram.api.(*mockBotAPI),
		)
		if !strings.Contains(
			edit.Text,
			materialReviewModeLabel(mode),
		) ||
			!strings.Contains(
				edit.Text,
				"새로 만드는 세션부터 적용",
			) ||
			!strings.Contains(
				edit.Text,
				"원래 Quiz 메시지",
			) ||
			edit.ReplyMarkup != nil {
			t.Fatalf(
				"Quiz selection did not close and confirm %s: %+v",
				mode,
				edit,
			)
		}
	}
	if repo.setCalls != 2 || repo.items[[2]int64{42, 10}].ReviewMode != model.MaterialReviewExcluded {
		t.Fatalf(
			"Quiz preference choices did not update linked material: %+v",
			repo,
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

func TestQuizMaterialPreferenceButtonFollowsLinkedQuestions(t *testing.T) {
	bot, _, _ := newPreferenceQuizBot(t)
	flow := NewSessionFlow(bot)
	flow.showQuestion(
		context.Background(),
		42,
		nil,
		77,
		0,
	)
	api := bot.telegram.api.(*mockBotAPI)
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
	b, repo, stateStores, activeRepo := newPreferenceStudyBot(t)
	flow := NewStudyFlow(b)
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
		if activeRepo.flushedState != nil {
			t.Fatalf(
				"%s flushed Study progress",
				data,
			)
		}
	}
	if repo.getCalls != 1 || repo.setCalls != 3 || len(repo.items) != 0 {
		t.Fatalf(
			"unexpected preference operations: %+v",
			repo,
		)
	}
	edit := lastEditMessage(
		t,
		b.telegram.api.(*mockBotAPI),
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
	for _, tc := range []struct {
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
			tc.name,
			func(t *testing.T) {
				b, repo, _, _ := newPreferenceStudyBot(t)
				NewStudyFlow(b).HandleCallback(
					context.Background(),
					preferenceCallback(
						tc.data,
						tc.user,
					),
				)
				if repo.getCalls != 0 || repo.setCalls != 0 {
					t.Fatalf(
						"unauthorized access reached preference service: %+v",
						repo,
					)
				}
			},
		)
	}
}

func TestStudyMaterialPreferenceEscapesHTMLAndExplainsMaintenance(t *testing.T) {
	b, _, _, _ := newPreferenceStudyBot(t)
	NewStudyFlow(b).HandleCallback(
		context.Background(),
		preferenceCallback(
			"study:77:policy:10:maintenance",
			42,
		),
	)
	edit := lastEditMessage(
		t,
		b.telegram.api.(*mockBotAPI),
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
	for _, mode := range []model.MaterialReviewMode{model.MaterialReviewNormal, model.MaterialReviewMaintenance, model.MaterialReviewExcluded} {
		if !hasPreferenceCallback(
			edit.ReplyMarkup,
			fmt.Sprintf(
				formatStudyPolicySet,
				77,
				10,
				mode,
			),
		) {
			t.Errorf(
				"missing policy %s",
				mode,
			)
		}
	}
}

func TestMaterialPreferencesListPaginationAndUserScopedRestore(t *testing.T) {
	b, _, api := newSettingsTestBot()
	repo := &botMaterialPreferenceRepo{items: make(map[[2]int64]model.MaterialPreference)}
	for materialID := 1; materialID <= 9; materialID++ {
		repo.items[[2]int64{42, int64(materialID)}] = model.MaterialPreference{
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
	repo.items[otherKey] = model.MaterialPreference{
		UserID:        43,
		MaterialID:    9,
		MaterialTitle: "other-user-secret",
		ReviewMode:    model.MaterialReviewExcluded,
	}
	b.services.MaterialPreference = service.NewMaterialPreferenceService(repo)
	b.handleSettingsCallback(
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
	if repo.listCalls != 1 || repo.listUser != 42 || repo.listStart != 0 || repo.listLimit != 9 || repo.getCalls != 0 {
		t.Fatalf(
			"expected one user-scoped batch query: %+v",
			repo,
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
	b.handleSettingsCallback(
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
	if repo.listStart != 8 || !strings.Contains(
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
		b.handleSettingsCallback(
			context.Background(),
			preferenceCallback(
				"settings:restore:9:1",
				42,
			),
		)
	}
	if _, ok := repo.items[[2]int64{42, 9}]; ok {
		t.Fatal("current user's preference was not restored")
	}
	if repo.items[otherKey].ReviewMode != model.MaterialReviewExcluded {
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
	b, repo, _, _ := newPreferenceStudyBot(t)
	for _, data := range []string{
		"study:77:policy", "study:77:policy:0", "study:77:policy:-1", "study:77:policy:10:bogus",
		"study:77:policy:10:normal:extra", "study:77:policy:10:", "study:0:policy:10:normal",
		"study:-1:policy:10:normal", "study:77:policy:+10:normal", "study:77:card:10:normal",
		"study:77:policy:99999999999999999999:normal", "study:77:policy:10:" + strings.Repeat(
			"x",
			65,
		),
	} {
		NewStudyFlow(b).HandleCallback(
			context.Background(),
			preferenceCallback(
				data,
				42,
			),
		)
	}
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
		b.handleSettingsCallback(
			context.Background(),
			preferenceCallback(
				data,
				42,
			),
		)
	}
	if repo.getCalls != 0 || repo.setCalls != 0 || repo.listCalls != 0 {
		t.Fatalf(
			"malformed callback reached preference repository: %+v",
			repo,
		)
	}
}

func TestMaterialPreferenceKeyboardAvailabilityAndCallbackLength(t *testing.T) {
	b, _, _ := newSettingsTestBot()
	keyboard := b.settingsKeyboard(&model.User{})
	if hasPreferenceCallback(
		&keyboard,
		"settings:materials:0",
	) {
		t.Fatal("preference menu shown without service")
	}
	b.services.MaterialPreference = service.NewMaterialPreferenceService(&botMaterialPreferenceRepo{})
	keyboard = b.settingsKeyboard(&model.User{})
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
	keyboards := []tgbotapi.InlineKeyboardMarkup{materialPreferenceKeyboard(
		maxID,
		maxID,
		model.MaterialReviewNormal,
	)}
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
