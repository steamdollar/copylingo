package bot

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type botStudyMaterialStore struct{}

func (s *botStudyMaterialStore) GetForStudySession(
	ctx context.Context,
	userID int64,
	language, level string,
	limit int,
) ([]model.Material, error) {
	return nil, nil
}

type botStudySessionStore struct {
	session    *model.Session
	started    []int
	completed  []int
	correctCnt []int
}

func (s *botStudySessionStore) CreateSession(ctx context.Context, session *model.Session) error {
	return nil
}

func (s *botStudySessionStore) GetByID(ctx context.Context, id int) (*model.Session, error) {
	return s.session, nil
}

func (s *botStudySessionStore) Start(ctx context.Context, id int) error {
	s.started = append(s.started, id)
	if s.session != nil {
		s.session.Status = model.SessionInProgress
	}
	return nil
}

func (s *botStudySessionStore) Complete(ctx context.Context, id int, correctCount int) error {
	s.completed = append(s.completed, id)
	s.correctCnt = append(s.correctCnt, correctCount)
	if s.session != nil {
		s.session.Status = model.SessionCompleted
	}
	return nil
}

type botStudySessionMaterialStore struct {
	items []model.StudySessionMaterial
}

func (s *botStudySessionMaterialStore) CreateSessionMaterials(ctx context.Context, sms []model.SessionMaterial) error {
	return nil
}

type botStudyActiveRepo struct {
	session      *model.Session
	items        []model.StudySessionMaterial
	flushedState *model.StudyActiveSessionState
}

func (r *botStudyActiveRepo) LoadStudySessionWithStateBySessionID(
	ctx context.Context,
	sessionID int,
) (*model.StudyActiveSessionState, error) {
	return &model.StudyActiveSessionState{
		Version:   model.StudyActiveSessionStateVersion,
		Session:   *r.session,
		Items:     r.items,
		UpdatedAt: timeNowForTest(),
	}, nil
}

func (r *botStudyActiveRepo) FlushStudyActiveSession(ctx context.Context, state *model.StudyActiveSessionState) error {
	r.flushedState = state
	if r.session != nil {
		r.session.Status = model.SessionCompleted
	}
	return nil
}

func TestStudyFlowStartNextFinish(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	api := &mockBotAPI{}
	sessionStore := &botStudySessionStore{
		session: &model.Session{
			ID:     sessionID,
			UserID: userID,
			Type:   model.SessionStudy,
			Mode:   model.SessionModeStudy,
			Status: model.SessionPending,
		},
	}
	items := []model.StudySessionMaterial{
		studyItem(sessionID, 10, 0, "みず", vocabularyPayload("みず", "水", "물", "noun")),
		studyItem(sessionID, 11, 1, "ひと", vocabularyPayload("ひと", "人", "사람", "noun")),
	}
	activeRepo := &botStudyActiveRepo{
		session: sessionStore.session,
		items:   items,
	}
	rdb := &testRedis{values: map[string]string{}}
	studyActiveService := service.NewStudyActiveSessionService(activeRepo, sessionStore, rdb)
	studyService := service.NewStudySessionService(
		&botStudyMaterialStore{},
		sessionStore,
		&botStudySessionMaterialStore{
			items: []model.StudySessionMaterial{
				studyItem(sessionID, 10, 0, "みず", vocabularyPayload("みず", "水", "물", "noun")),
				studyItem(sessionID, 11, 1, "ひと", vocabularyPayload("ひと", "人", "사람", "noun")),
			},
		},
	)
	b := &Bot{
		api: api,
		services: &service.Services{
			StudySession:       studyService,
			StudyActiveSession: studyActiveService,
		},
	}
	flow := NewStudyFlow(b)

	flow.HandleCallback(ctx, studyCallback(config.FormatStudyStart, sessionID, 0, userID))
	if len(sessionStore.started) != 1 || sessionStore.started[0] != sessionID {
		t.Fatalf("started = %+v, want [%d]", sessionStore.started, sessionID)
	}
	edit := lastEditMessage(t, api)
	if !strings.Contains(edit.Text, "1/2") || !strings.Contains(edit.Text, "みず") || !strings.Contains(edit.Text, "물") {
		t.Fatalf("start edit text = %q", edit.Text)
	}
	if got := onlyCallbackData(t, edit); got != "study:77:next:0" {
		t.Fatalf("start callback = %q", got)
	}

	flow.HandleCallback(ctx, studyCallback(config.FormatStudyNext, sessionID, 0, userID))
	state, err := studyActiveService.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Get active study state after next failed: %v", err)
	}
	if state.Items[0].SessionMaterial.StudiedAt == nil {
		t.Fatal("first material should be marked studied after next")
	}
	edit = lastEditMessage(t, api)
	if !strings.Contains(edit.Text, "2/2") || !strings.Contains(edit.Text, "ひと") || !strings.Contains(edit.Text, "사람") {
		t.Fatalf("next edit text = %q", edit.Text)
	}
	if got := rowCallbackData(t, edit); len(got) != 2 ||
		got[0] != "study:77:prev:1" || got[1] != "study:77:finish:1" {
		t.Fatalf("last card callbacks = %v, want [study:77:prev:1 study:77:finish:1]", got)
	}

	flow.HandleCallback(ctx, studyCallback(config.FormatStudyFinish, sessionID, 1, userID))
	if activeRepo.flushedState == nil {
		t.Fatal("expected study active session to flush on finish")
	}
	if activeRepo.flushedState.Items[0].SessionMaterial.StudiedAt == nil ||
		activeRepo.flushedState.Items[1].SessionMaterial.StudiedAt == nil {
		t.Fatalf("expected all materials studied before flush: %+v", activeRepo.flushedState.Items)
	}
	edit = lastEditMessage(t, api)
	if !strings.Contains(edit.Text, "완료") {
		t.Fatalf("finish edit text = %q", edit.Text)
	}
}

func TestStudyFlowPrevNavigation(t *testing.T) {
	ctx := context.Background()
	sessionID := 79
	userID := int64(123)
	api := &mockBotAPI{}
	sessionStore := &botStudySessionStore{
		session: &model.Session{
			ID:     sessionID,
			UserID: userID,
			Type:   model.SessionStudy,
			Mode:   model.SessionModeStudy,
			Status: model.SessionPending,
		},
	}
	items := []model.StudySessionMaterial{
		studyItem(sessionID, 10, 0, "みず", vocabularyPayload("みず", "水", "물", "noun")),
		studyItem(sessionID, 11, 1, "ひと", vocabularyPayload("ひと", "人", "사람", "noun")),
	}
	activeRepo := &botStudyActiveRepo{
		session: sessionStore.session,
		items:   items,
	}
	rdb := &testRedis{values: map[string]string{}}
	studyActiveService := service.NewStudyActiveSessionService(activeRepo, sessionStore, rdb)
	b := &Bot{
		api: api,
		services: &service.Services{
			StudyActiveSession: studyActiveService,
		},
	}
	flow := NewStudyFlow(b)

	flow.HandleCallback(ctx, studyCallback(config.FormatStudyStart, sessionID, 0, userID))
	flow.HandleCallback(ctx, studyCallback(config.FormatStudyNext, sessionID, 0, userID))

	// 마지막 카드(2/2)에서 이전으로 돌아가면 첫 카드가 다시 보인다.
	flow.HandleCallback(ctx, studyCallback(config.FormatStudyPrev, sessionID, 1, userID))
	edit := lastEditMessage(t, api)
	if !strings.Contains(edit.Text, "1/2") || !strings.Contains(edit.Text, "みず") {
		t.Fatalf("prev edit text = %q", edit.Text)
	}
	// 첫 카드에는 이전 버튼이 없다.
	if got := onlyCallbackData(t, edit); got != "study:79:next:0" {
		t.Fatalf("first card callback = %q", got)
	}

	// 뒤로 가기는 studied 상태를 되돌리지 않는다.
	state, err := studyActiveService.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Get active study state after prev failed: %v", err)
	}
	if state.Items[0].SessionMaterial.StudiedAt == nil {
		t.Fatal("first material should stay studied after prev")
	}

	// 이미 studied인 카드에서 다시 앞으로 이동한 뒤 완료까지 정상 진행된다.
	flow.HandleCallback(ctx, studyCallback(config.FormatStudyNext, sessionID, 0, userID))
	edit = lastEditMessage(t, api)
	if !strings.Contains(edit.Text, "2/2") || !strings.Contains(edit.Text, "ひと") {
		t.Fatalf("re-next edit text = %q", edit.Text)
	}
	flow.HandleCallback(ctx, studyCallback(config.FormatStudyFinish, sessionID, 1, userID))
	if activeRepo.flushedState == nil {
		t.Fatal("expected study active session to flush on finish after prev round-trip")
	}
}

func TestStudyFlowGrammarRendering(t *testing.T) {
	ctx := context.Background()
	sessionID := 78
	userID := int64(123)
	api := &mockBotAPI{}
	sessionStore := &botStudySessionStore{
		session: &model.Session{
			ID:     sessionID,
			UserID: userID,
			Type:   model.SessionStudy,
			Mode:   model.SessionModeStudy,
			Status: model.SessionPending,
		},
	}
	grammarPayload := func(pattern, meaningKo, explanationKo, example, exampleReading, translationKo string) json.RawMessage {
		payload, err := json.Marshal(map[string]string{
			"pattern":         pattern,
			"meaning_ko":      meaningKo,
			"explanation_ko":  explanationKo,
			"example":         example,
			"example_reading": exampleReading,
			"translation_ko":  translationKo,
		})
		if err != nil {
			panic(err)
		}
		return payload
	}
	items := []model.StudySessionMaterial{
		{
			SessionMaterial: model.SessionMaterial{
				SessionID:     sessionID,
				MaterialID:    20,
				MaterialOrder: 0,
			},
			Material: model.Material{
				ID:               20,
				Category:         model.MaterialCategoryGrammar,
				Language:         "ja",
				ProficiencyLevel: "N5",
				Title:            "は",
				Payload:          grammarPayload("は", "주제 표시", "이미 알고 있는 주제", "私は学生です。", "わたしはがくせいです。", "저는 학생입니다."),
			},
		},
	}
	activeRepo := &botStudyActiveRepo{
		session: sessionStore.session,
		items:   items,
	}
	rdb := &testRedis{values: map[string]string{}}
	studyActiveService := service.NewStudyActiveSessionService(activeRepo, sessionStore, rdb)
	studyService := service.NewStudySessionService(
		&botStudyMaterialStore{},
		sessionStore,
		&botStudySessionMaterialStore{items: items},
	)
	b := &Bot{
		api: api,
		services: &service.Services{
			StudySession:       studyService,
			StudyActiveSession: studyActiveService,
		},
	}
	flow := NewStudyFlow(b)

	flow.HandleCallback(ctx, studyCallback(config.FormatStudyStart, sessionID, 0, userID))
	edit := lastEditMessage(t, api)
	if !strings.Contains(edit.Text, "Grammar") || !strings.Contains(edit.Text, "주제 표시") ||
		!strings.Contains(edit.Text, "이미 알고 있는 주제") || !strings.Contains(edit.Text, "わたしはがくせいです。") {
		t.Fatalf("grammar start edit text = %q", edit.Text)
	}
}

func timeNowForTest() time.Time {
	return time.Now()
}

func studyItem(sessionID, materialID, order int, title string, payload json.RawMessage) model.StudySessionMaterial {
	return model.StudySessionMaterial{
		SessionMaterial: model.SessionMaterial{
			SessionID:     sessionID,
			MaterialID:    materialID,
			MaterialOrder: order,
		},
		Material: model.Material{
			ID:               materialID,
			Category:         model.MaterialCategoryVocabulary,
			Language:         "ja",
			ProficiencyLevel: "N5",
			Title:            title,
			Payload:          payload,
		},
	}
}

func vocabularyPayload(kana, kanji, meaningKo, partOfSpeech string) json.RawMessage {
	payload, err := json.Marshal(map[string]string{
		"kana":           kana,
		"kanji":          kanji,
		"meaning_ko":     meaningKo,
		"part_of_speech": partOfSpeech,
	})
	if err != nil {
		panic(err)
	}
	return payload
}

func studyCallback(format string, sessionID, order int, userID int64) *tgbotapi.CallbackQuery {
	data := ""
	switch format {
	case config.FormatStudyStart:
		data = "study:" + intString(sessionID) + ":start"
	case config.FormatStudyNext:
		data = "study:" + intString(sessionID) + ":next:" + intString(order)
	case config.FormatStudyPrev:
		data = "study:" + intString(sessionID) + ":prev:" + intString(order)
	case config.FormatStudyFinish:
		data = "study:" + intString(sessionID) + ":finish:" + intString(order)
	}
	return &tgbotapi.CallbackQuery{
		Data: data,
		From: &tgbotapi.User{ID: userID},
		Message: &tgbotapi.Message{
			MessageID: 456,
			Chat:      &tgbotapi.Chat{ID: userID},
		},
	}
}

func intString(v int) string {
	return strconv.Itoa(v)
}

func lastEditMessage(t *testing.T, api *mockBotAPI) tgbotapi.EditMessageTextConfig {
	t.Helper()
	if len(api.sentMessages) == 0 {
		t.Fatal("no Telegram messages sent")
	}
	edit, ok := api.sentMessages[len(api.sentMessages)-1].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf("last message type = %T, want EditMessageTextConfig", api.sentMessages[len(api.sentMessages)-1])
	}
	return edit
}

func onlyCallbackData(t *testing.T, edit tgbotapi.EditMessageTextConfig) string {
	t.Helper()
	datas := rowCallbackData(t, edit)
	if len(datas) != 1 {
		t.Fatalf("keyboard row buttons = %v, want exactly 1", datas)
	}
	return datas[0]
}

func rowCallbackData(t *testing.T, edit tgbotapi.EditMessageTextConfig) []string {
	t.Helper()
	if edit.ReplyMarkup == nil || len(edit.ReplyMarkup.InlineKeyboard) != 1 {
		t.Fatalf("unexpected reply markup: %+v", edit.ReplyMarkup)
	}
	row := edit.ReplyMarkup.InlineKeyboard[0]
	datas := make([]string, 0, len(row))
	for _, button := range row {
		if button.CallbackData == nil {
			t.Fatal("callback data is nil")
		}
		datas = append(datas, *button.CallbackData)
	}
	return datas
}
