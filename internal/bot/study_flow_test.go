package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type botStudyMaterialStore struct{}

func (store *botStudyMaterialStore) GetMaterialsByPlan(
	ctx context.Context,
	userID int64,
	language,
	level string,
	levels []string,
	plan model.StudySessionPlan,
) ([]model.Material, error) {
	return nil, nil
}

// The embedded nil SessionRepository makes any other sessions-table call panic.
type botStudySessionStore struct {
	service.SessionRepository
	session      *model.Session
	started      []int
	completed    []int
	correctCount []int
}

func (store *botStudySessionStore) CreateSessionInTx(
	ctx context.Context,
	tx *sqlx.Tx,
	session *model.Session,
) (int, error) {
	return 0, nil
}

func (store *botStudySessionStore) CreateSessionMaterialsInTx(
	ctx context.Context,
	tx *sqlx.Tx,
	sessionID int,
	materialIDs []int,
) error {
	return nil
}

func (store *botStudySessionStore) GetByID(
	ctx context.Context,
	id int,
) (*model.Session, error) {
	return store.session, nil
}

func (store *botStudySessionStore) Start(
	ctx context.Context,
	id int,
) error {
	store.started = append(
		store.started,
		id,
	)
	if store.session != nil {
		store.session.Status = model.SessionInProgress
	}
	return nil
}

func (store *botStudySessionStore) Complete(
	ctx context.Context,
	id int,
	correctCount int,
) error {
	store.completed = append(
		store.completed,
		id,
	)
	store.correctCount = append(
		store.correctCount,
		correctCount,
	)
	if store.session != nil {
		store.session.Status = model.SessionCompleted
	}
	return nil
}

type botStudyActiveRepository struct {
	session      *model.Session
	items        []model.StudySessionMaterial
	flushedState *model.StudyActiveSessionState
}

func (repository *botStudyActiveRepository) LoadStudySessionWithStateBySessionID(
	ctx context.Context,
	sessionID int,
) (*model.StudyActiveSessionState, error) {
	return &model.StudyActiveSessionState{
		Session:   *repository.session,
		Items:     repository.items,
		UpdatedAt: timeNowForTest(),
	}, nil
}

func (repository *botStudyActiveRepository) FlushStudyActiveSession(
	ctx context.Context,
	state *model.StudyActiveSessionState,
) error {
	repository.flushedState = state
	if repository.session != nil {
		repository.session.Status = model.SessionCompleted
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
		studyItem(
			sessionID,
			10,
			0,
			"みず",
			vocabularyPayload(
				"みず",
				"水",
				"물",
				"noun",
			),
		),
		studyItem(
			sessionID,
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
	}
	activeRepository := &botStudyActiveRepository{
		session: sessionStore.session,
		items:   items,
	}
	stateStores := newTestInteractionStores()
	session := newTestSessionService(
		stateStores,
		service.SessionDependencies{
			MaterialRepository:           &botStudyMaterialStore{},
			SessionRepository:            sessionStore,
			StudyActiveSessionRepository: activeRepository,
			DB:                           botTestDB,
		},
	)
	flow := newTestStudyFlow(
		api,
		nil,
		StudyFlowDependencies{
			Session: session,
		},
	)

	flow.HandleCallback(
		ctx,
		studyCallback(
			formatStudyStart,
			sessionID,
			0,
			userID,
		),
	)
	if len(sessionStore.started) != 1 || sessionStore.started[0] != sessionID {
		t.Fatalf(
			"started = %+v, want [%d]",
			sessionStore.started,
			sessionID,
		)
	}
	edit := lastEditMessage(
		t,
		api,
	)
	if !strings.Contains(
		edit.Text,
		"1/2",
	) || !strings.Contains(
		edit.Text,
		"みず",
	) || !strings.Contains(
		edit.Text,
		"물",
	) {
		t.Fatalf(
			"start edit text = %q",
			edit.Text,
		)
	}
	if got := onlyCallbackData(
		t,
		edit,
	); got != "study:77:next:0" {
		t.Fatalf(
			"start callback = %q",
			got,
		)
	}

	flow.HandleCallback(
		ctx,
		studyCallback(
			formatStudyNext,
			sessionID,
			0,
			userID,
		),
	)
	state, err := session.StudyProgress(
		ctx,
		sessionID,
		userID,
	)
	if err != nil {
		t.Fatalf(
			"Get active study state after next failed: %v",
			err,
		)
	}
	if state.Items[0].SessionMaterial.StudiedAt == nil {
		t.Fatal("first material should be marked studied after next")
	}
	edit = lastEditMessage(
		t,
		api,
	)
	if !strings.Contains(
		edit.Text,
		"2/2",
	) || !strings.Contains(
		edit.Text,
		"ひと",
	) || !strings.Contains(
		edit.Text,
		"사람",
	) {
		t.Fatalf(
			"next edit text = %q",
			edit.Text,
		)
	}
	if got := rowCallbackData(
		t,
		edit,
	); len(got) != 2 ||
		got[0] != "study:77:prev:1" || got[1] != "study:77:finish:1" {
		t.Fatalf(
			"last card callbacks = %v, want [study:77:prev:1 study:77:finish:1]",
			got,
		)
	}

	flow.HandleCallback(
		ctx,
		studyCallback(
			formatStudyFinish,
			sessionID,
			1,
			userID,
		),
	)
	if activeRepository.flushedState == nil {
		t.Fatal("expected study active session to flush on finish")
	}
	if activeRepository.flushedState.Items[0].SessionMaterial.StudiedAt == nil ||
		activeRepository.flushedState.Items[1].SessionMaterial.StudiedAt == nil {
		t.Fatalf(
			"expected all materials studied before flush: %+v",
			activeRepository.flushedState.Items,
		)
	}
	edit = lastEditMessage(
		t,
		api,
	)
	if !strings.Contains(
		edit.Text,
		"완료",
	) {
		t.Fatalf(
			"finish edit text = %q",
			edit.Text,
		)
	}
}

func TestStudyFlowPreviousNavigation(t *testing.T) {
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
		studyItem(
			sessionID,
			10,
			0,
			"みず",
			vocabularyPayload(
				"みず",
				"水",
				"물",
				"noun",
			),
		),
		studyItem(
			sessionID,
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
	}
	activeRepository := &botStudyActiveRepository{
		session: sessionStore.session,
		items:   items,
	}
	stateStores := newTestInteractionStores()
	session := newTestSessionService(
		stateStores,
		service.SessionDependencies{
			SessionRepository:            sessionStore,
			StudyActiveSessionRepository: activeRepository,
		},
	)
	flow := newTestStudyFlow(
		api,
		nil,
		StudyFlowDependencies{
			Session: session,
		},
	)

	flow.HandleCallback(
		ctx,
		studyCallback(
			formatStudyStart,
			sessionID,
			0,
			userID,
		),
	)
	flow.HandleCallback(
		ctx,
		studyCallback(
			formatStudyNext,
			sessionID,
			0,
			userID,
		),
	)

	flow.HandleCallback(
		ctx,
		studyCallback(
			formatStudyPrevious,
			sessionID,
			1,
			userID,
		),
	)
	edit := lastEditMessage(
		t,
		api,
	)
	if !strings.Contains(
		edit.Text,
		"1/2",
	) || !strings.Contains(
		edit.Text,
		"みず",
	) {
		t.Fatalf(
			"prev edit text = %q",
			edit.Text,
		)
	}
	if got := onlyCallbackData(
		t,
		edit,
	); got != "study:79:next:0" {
		t.Fatalf(
			"first card callback = %q",
			got,
		)
	}

	// 뒤로 가기는 studied 상태를 되돌리지 않는다.
	state, err := session.StudyProgress(
		ctx,
		sessionID,
		userID,
	)
	if err != nil {
		t.Fatalf(
			"Get active study state after prev failed: %v",
			err,
		)
	}
	if state.Items[0].SessionMaterial.StudiedAt == nil {
		t.Fatal("first material should stay studied after prev")
	}

	flow.HandleCallback(
		ctx,
		studyCallback(
			formatStudyNext,
			sessionID,
			0,
			userID,
		),
	)
	edit = lastEditMessage(
		t,
		api,
	)
	if !strings.Contains(
		edit.Text,
		"2/2",
	) || !strings.Contains(
		edit.Text,
		"ひと",
	) {
		t.Fatalf(
			"re-next edit text = %q",
			edit.Text,
		)
	}
	flow.HandleCallback(
		ctx,
		studyCallback(
			formatStudyFinish,
			sessionID,
			1,
			userID,
		),
	)
	if activeRepository.flushedState == nil {
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
	grammarPayload := func(
		pattern,
		meaningKo,
		explanationKo,
		example,
		exampleReading,
		translationKo string,
	) json.RawMessage {
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
				Payload: grammarPayload(
					"は",
					"주제 표시",
					"이미 알고 있는 주제",
					"私は学生です。",
					"わたしはがくせいです。",
					"저는 학생입니다.",
				),
			},
		},
	}
	activeRepository := &botStudyActiveRepository{
		session: sessionStore.session,
		items:   items,
	}
	stateStores := newTestInteractionStores()
	flow := newTestStudyFlow(
		api,
		nil,
		StudyFlowDependencies{
			Session: newTestSessionService(
				stateStores,
				service.SessionDependencies{
					MaterialRepository:           &botStudyMaterialStore{},
					SessionRepository:            sessionStore,
					StudyActiveSessionRepository: activeRepository,
					DB:                           botTestDB,
				},
			),
		},
	)

	flow.HandleCallback(
		ctx,
		studyCallback(
			formatStudyStart,
			sessionID,
			0,
			userID,
		),
	)
	edit := lastEditMessage(
		t,
		api,
	)
	if !strings.Contains(
		edit.Text,
		"Grammar",
	) || !strings.Contains(
		edit.Text,
		"주제 표시",
	) ||
		!strings.Contains(
			edit.Text,
			"이미 알고 있는 주제",
		) || !strings.Contains(
		edit.Text,
		"わたしはがくせいです。",
	) {
		t.Fatalf(
			"grammar start edit text = %q",
			edit.Text,
		)
	}
}

func TestStudyFlowReadingRendering(t *testing.T) {
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
	readingPayload, err := json.Marshal(map[string]any{
		// The "<b>" literal must come out HTML-escaped, not parsed as a tag.
		"passage": "図書館は毎週月曜日が休みです。<b>テスト&検証</b>",
		"reading": "としょかんはまいしゅうげつようびがやすみです。",
		"key_vocabulary": []map[string]string{
			{"surface": "図書館", "reading": "としょかん", "meaning_ko": "도서관"},
			{"surface": "くだもの", "reading": "くだもの", "meaning_ko": "과일"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	items := []model.StudySessionMaterial{
		{
			SessionMaterial: model.SessionMaterial{
				SessionID:     sessionID,
				MaterialID:    30,
				MaterialOrder: 0,
			},
			Material: model.Material{
				ID:               30,
				Category:         model.MaterialCategoryReading,
				Language:         "ja",
				ProficiencyLevel: "N5",
				Title:            "図書館のお知らせ",
				Payload:          readingPayload,
			},
		},
	}
	activeRepository := &botStudyActiveRepository{
		session: sessionStore.session,
		items:   items,
	}
	stateStores := newTestInteractionStores()
	flow := newTestStudyFlow(
		api,
		nil,
		StudyFlowDependencies{
			Session: newTestSessionService(
				stateStores,
				service.SessionDependencies{
					MaterialRepository:           &botStudyMaterialStore{},
					SessionRepository:            sessionStore,
					StudyActiveSessionRepository: activeRepository,
					DB:                           botTestDB,
				},
			),
		},
	)

	flow.HandleCallback(
		ctx,
		studyCallback(
			formatStudyStart,
			sessionID,
			0,
			userID,
		),
	)
	edit := lastEditMessage(
		t,
		api,
	)
	for _, want := range []string{
		"Reading",
		"図書館のお知らせ",
		"図書館は毎週月曜日が休みです。",
		"읽기: としょかんはまいしゅうげつようびがやすみです。",
		"핵심 어휘:",
		"도서관",
	} {
		if !strings.Contains(
			edit.Text,
			want,
		) {
			t.Fatalf(
				"reading start edit text missing %q: %q",
				want,
				edit.Text,
			)
		}
	}
	if !strings.Contains(
		edit.Text,
		"&lt;b&gt;テスト&amp;検証&lt;/b&gt;",
	) {
		t.Fatalf(
			"reading passage is not HTML-escaped: %q",
			edit.Text,
		)
	}
	// Same-as-surface readings (e.g. kana-only words) must not render "(くだもの)" twice.
	if strings.Contains(
		edit.Text,
		"くだもの (くだもの)",
	) {
		t.Fatalf(
			"kana-only vocabulary repeats its reading: %q",
			edit.Text,
		)
	}
}

func TestRenderReadingPayloadFallbacks(t *testing.T) {
	t.Parallel()

	if got := renderReadingPayload(json.RawMessage(`{}`)); got != "" {
		t.Fatalf(
			"empty reading payload = %q, want empty string",
			got,
		)
	}
	// Malformed payloads fall back to the generic <pre> renderer.
	if got := renderReadingPayload(json.RawMessage(`{"passage": 1}`)); !strings.Contains(
		got,
		"<pre>",
	) {
		t.Fatalf(
			"malformed reading payload = %q, want generic fallback",
			got,
		)
	}
}

func TestStudyMaterialKeyboardAskLLMGate(t *testing.T) {
	t.Parallel()

	askData := fmt.Sprintf(
		formatStudyAskLLM,
		99,
		3,
	)
	for _, testCase := range []struct {
		name    string
		showAsk bool
		wantAsk bool
	}{
		{name: "owner sees ask button", showAsk: true, wantAsk: true},
		{name: "non-owner has no ask button", showAsk: false, wantAsk: false},
	} {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				keyboard := studyMaterialKeyboard(
					99,
					3,
					false,
					false,
					testCase.showAsk,
				)
				found := false
				for _, row := range keyboard.InlineKeyboard {
					for _, button := range row {
						if button.CallbackData != nil && *button.CallbackData == askData {
							found = true
						}
					}
				}
				if found != testCase.wantAsk {
					t.Errorf(
						"ask button present = %t, want %t",
						found,
						testCase.wantAsk,
					)
				}
			},
		)
	}
}

func timeNowForTest() time.Time {
	return time.Now()
}

func studyItem(
	sessionID,
	materialID,
	order int,
	title string,
	payload json.RawMessage,
) model.StudySessionMaterial {
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

func vocabularyPayload(
	kana,
	kanji,
	meaningKo,
	partOfSpeech string,
) json.RawMessage {
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

func studyCallback(
	format string,
	sessionID,
	order int,
	userID int64,
) *tgbotapi.CallbackQuery {
	data := ""
	switch format {
	case formatStudyStart:
		data = "study:" + intString(sessionID) + ":start"
	case formatStudyNext:
		data = "study:" + intString(sessionID) + ":next:" + intString(order)
	case formatStudyPrevious:
		data = "study:" + intString(sessionID) + ":prev:" + intString(order)
	case formatStudyFinish:
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

func intString(value int) string {
	return strconv.Itoa(value)
}

func lastEditMessage(
	t *testing.T,
	api *mockBotAPI,
) tgbotapi.EditMessageTextConfig {
	t.Helper()
	if len(api.sentMessages) == 0 {
		t.Fatal("no Telegram messages sent")
	}
	edit, ok := api.sentMessages[len(api.sentMessages)-1].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf(
			"last message type = %T, want EditMessageTextConfig",
			api.sentMessages[len(api.sentMessages)-1],
		)
	}
	return edit
}

func onlyCallbackData(
	t *testing.T,
	edit tgbotapi.EditMessageTextConfig,
) string {
	t.Helper()
	datas := rowCallbackData(
		t,
		edit,
	)
	if len(datas) != 1 {
		t.Fatalf(
			"keyboard row buttons = %v, want exactly 1",
			datas,
		)
	}
	return datas[0]
}

func rowCallbackData(
	t *testing.T,
	edit tgbotapi.EditMessageTextConfig,
) []string {
	t.Helper()
	// Only the first (navigation) row is inspected.
	if edit.ReplyMarkup == nil || len(edit.ReplyMarkup.InlineKeyboard) != 2 {
		t.Fatalf(
			"unexpected reply markup: %+v",
			edit.ReplyMarkup,
		)
	}
	row := edit.ReplyMarkup.InlineKeyboard[0]
	datas := make(
		[]string,
		0,
		len(row),
	)
	for _, button := range row {
		if button.CallbackData == nil {
			t.Fatal("callback data is nil")
		}
		datas = append(
			datas,
			*button.CallbackData,
		)
	}
	return datas
}

// 清 has a kanji note and readings, 潔 has readings but no note.
const seiketsuPayload = `{"kana":"せいけつ","kanji":"清潔","meaning_ko":"청결함","part_of_speech":"noun",` +
	`"kanji_readings":[{"text":"清","reading":"せい","type":"on","on":["セイ","ショウ"]},` +
	`{"text":"潔","reading":"けつ","type":"on","on":["ケツ"]}],` +
	`"kanji_notes":[{"char":"清","type":"phono_semantic",` +
	`"parts":[{"part":"氵","role":"meaning","gloss_ko":"물"},{"part":"青","role":"sound","on":"セイ"}],` +
	`"explanation_ko":"물(氵)처럼 맑다는 뜻에 青이 음 セイ를 더한다."}]}`

func TestHasKanjiDetailDecidesButton(t *testing.T) {
	t.Parallel()

	base := `"kana":"せいけつ","kanji":"清潔","meaning_ko":"청결함","part_of_speech":"noun"`
	for _, testCase := range []struct {
		name     string
		category model.MaterialCategory
		payload  string
		want     bool
	}{
		{name: "notes and readings", category: model.MaterialCategoryVocabulary, payload: seiketsuPayload, want: true},
		{
			name:     "readings only",
			category: model.MaterialCategoryVocabulary,
			payload:  "{" + base + `,"kanji_readings":[{"text":"清","reading":"せい","type":"on"}]}`,
			want:     true,
		},
		{name: "absent", category: model.MaterialCategoryVocabulary, payload: "{" + base + "}", want: false},
		{
			name:     "null",
			category: model.MaterialCategoryVocabulary,
			payload:  "{" + base + `,"kanji_notes":null,"kanji_readings":null}`,
			want:     false,
		},
		{
			name:     "empty",
			category: model.MaterialCategoryVocabulary,
			payload:  "{" + base + `,"kanji_notes":[],"kanji_readings":[]}`,
			want:     false,
		},
		{name: "not vocabulary", category: model.MaterialCategoryGrammar, payload: seiketsuPayload, want: false},
	} {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				got := hasKanjiDetail(model.Material{
					Category: testCase.category,
					Payload:  json.RawMessage(testCase.payload),
				})
				if got != testCase.want {
					t.Errorf(
						"hasKanjiDetail = %v, want %v",
						got,
						testCase.want,
					)
				}
			},
		)
	}
}

func TestRenderStudyMaterialIgnoresKanjiNotes(t *testing.T) {
	t.Parallel()

	card := func(payload json.RawMessage) string {
		return renderStudyMaterial(
			model.Material{
				Category: model.MaterialCategoryVocabulary,
				Title:    "せいけつ",
				Payload:  payload,
			},
			0,
			1,
		)
	}
	want := card(vocabularyPayload(
		"せいけつ",
		"清潔",
		"청결함",
		"noun",
	))
	if got := card(json.RawMessage(seiketsuPayload)); got != want {
		t.Errorf(
			"card with notes = %q, want %q",
			got,
			want,
		)
	}
}

func TestRenderStudyMaterialLevelMarker(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name  string
		level string
		want  string
	}{
		{
			name:  "with level",
			level: "N4",
			want:  "<b>1/3 · N4 · Vocabulary</b>",
		},
		{
			name:  "without level",
			level: "",
			want:  "<b>1/3 · Vocabulary</b>",
		},
	} {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				t.Parallel()

				got := renderStudyMaterial(
					model.Material{
						Category:         model.MaterialCategoryVocabulary,
						ProficiencyLevel: testCase.level,
						Title:            "みず",
						Payload: vocabularyPayload(
							"みず",
							"水",
							"물",
							"noun",
						),
					},
					0,
					3,
				)
				if !strings.Contains(
					got,
					testCase.want,
				) {
					t.Errorf(
						"card = %q, want header %q",
						got,
						testCase.want,
					)
				}
			},
		)
	}
}

func TestRenderKanjiNotes(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name       string
		vocabulary vocabularyStudyPayload
		want       []string
		notWant    []string
	}{
		{
			name: "kun reading below the note",
			vocabulary: vocabularyStudyPayload{
				Kana:  "まつ",
				Kanji: "待つ",
				KanjiReadings: []model.KanjiReading{
					{Text: "待", Reading: "ま", Type: model.KanjiReadingKun, On: []string{"タイ"}, HunEum: "기다릴 대"},
				},
				KanjiNotes: []model.KanjiNote{
					{
						Character: "待",
						Type:      model.KanjiNotePhonoSemantic,
						Parts: []model.KanjiPart{
							{Part: "彳", Role: model.KanjiPartMeaning, GlossKo: "길을 감"},
							{Part: "寺", Role: model.KanjiPartSound, HunEum: "절 사"},
						},
						ExplanationKo: "寺(ジ)는 원래 발음 요소지만 일본어 음은 タイ다.",
					},
					{
						Character: "読",
						Type:      model.KanjiNotePhonoSemantic,
						Parts: []model.KanjiPart{
							{Part: "言", Role: model.KanjiPartMeaning, GlossKo: "말"},
							{Part: "売", Role: model.KanjiPartForm, GlossKo: "간략화된 <모양>"},
						},
						ExplanationKo: "구자체 讀.",
					},
				},
			},
			want: []string{
				"<b>待つ</b> (まつ)",
				"<b>待</b> 기다릴 대 · 형성문자\n彳(길을 감) + 寺(절 사 · 발음)\n寺(ジ)는 원래 발음 요소지만 일본어 음은 タイ다.\n" +
					"이 단어에선: ま (훈독)\n이 한자의 음독: タイ",
				"<b>読</b> · 형성문자\n言(말) + 売(간략화된 &lt;모양&gt;)\n구자체 讀.",
			},
		},
		{
			name: "radical base and sound part with on'yomi",
			vocabulary: vocabularyStudyPayload{
				Kana:  "ゆび",
				Kanji: "指",
				KanjiReadings: []model.KanjiReading{
					{Text: "指", Reading: "ゆび", Type: model.KanjiReadingKun, On: []string{"シ"}, HunEum: "가리킬 지"},
				},
				KanjiNotes: []model.KanjiNote{
					{
						Character: "指",
						Type:      model.KanjiNotePhonoSemantic,
						Parts: []model.KanjiPart{
							{Part: "扌", Role: model.KanjiPartMeaning, GlossKo: "손", Base: "手"},
							{Part: "旨", Role: model.KanjiPartSound, HunEum: "뜻 지", On: "シ"},
						},
						ExplanationKo: "손(扌)으로 가리킨다는 뜻이다.",
					},
				},
			},
			want: []string{"扌(手 · 손) + 旨(뜻 지 · シ · 발음)"},
		},
		{
			name: "kanji without a note",
			vocabulary: vocabularyStudyPayload{
				Kana:  "じこ",
				Kanji: "事故",
				KanjiReadings: []model.KanjiReading{
					{Text: "事", Reading: "じ", Type: model.KanjiReadingOn, On: []string{"ジ", "ズ"}, HunEum: "일 사"},
					{Text: "故", Reading: "こ", Type: model.KanjiReadingOn, On: []string{"コ"}},
				},
			},
			want: []string{
				"<b>事</b> 일 사\n이 단어에선: じ (음독)\n이 한자의 음독: ジ · ズ",
				"<b>故</b>\n이 단어에선: こ (음독)\n이 한자의 음독: コ",
			},
		},
		{
			name: "jukuji group",
			vocabulary: vocabularyStudyPayload{
				Kana:  "きょう",
				Kanji: "今日",
				KanjiReadings: []model.KanjiReading{
					{Text: "今日", Reading: "きょう", Type: model.KanjiReadingJukuji, HunEum: "今 이제 금 · 日 날 일"},
				},
			},
			want:    []string{"<b>今日</b> 今 이제 금 · 日 날 일\n이 단어에선: きょう (단어 전체 읽기)"},
			notWant: []string{"이 한자의 음독"},
		},
	} {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				got := renderKanjiNotes(testCase.vocabulary)
				for _, want := range testCase.want {
					if !strings.Contains(
						got,
						want,
					) {
						t.Errorf(
							"kanji notes missing %q:\n%s",
							want,
							got,
						)
					}
				}
				for _, notWant := range testCase.notWant {
					if strings.Contains(
						got,
						notWant,
					) {
						t.Errorf(
							"kanji notes has %q:\n%s",
							notWant,
							got,
						)
					}
				}
			},
		)
	}
}

func TestStudyFlowKanjiNotes(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	// The owner sees the ask-LLM button, so the row order can be checked.
	userID := llmAllowedTelegramUserIDs[0]
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
		studyItem(
			sessionID,
			10,
			0,
			"せいけつ",
			json.RawMessage(seiketsuPayload),
		),
		studyItem(
			sessionID,
			11,
			1,
			"みず",
			vocabularyPayload(
				"みず",
				"水",
				"물",
				"noun",
			),
		),
	}
	session := newTestSessionService(
		newTestInteractionStores(),
		service.SessionDependencies{
			MaterialRepository: &botStudyMaterialStore{},
			SessionRepository:  sessionStore,
			StudyActiveSessionRepository: &botStudyActiveRepository{
				session: sessionStore.session,
				items:   items,
			},
			DB: botTestDB,
		},
	)
	flow := newTestStudyFlow(
		api,
		nil,
		StudyFlowDependencies{Session: session},
	)
	kanjiCallback := func(materialID int) *tgbotapi.CallbackQuery {
		return &tgbotapi.CallbackQuery{
			Data: fmt.Sprintf(
				formatStudyKanji,
				sessionID,
				materialID,
			),
			From: &tgbotapi.User{ID: userID},
			Message: &tgbotapi.Message{
				MessageID: 456,
				Chat:      &tgbotapi.Chat{ID: userID},
			},
		}
	}
	kanjiData := fmt.Sprintf(
		formatStudyKanji,
		sessionID,
		10,
	)

	flow.HandleCallback(
		ctx,
		studyCallback(
			formatStudyStart,
			sessionID,
			0,
			userID,
		),
	)
	cardData := allCallbackData(lastEditMessage(
		t,
		api,
	))
	kanjiIndex := slices.Index(
		cardData,
		kanjiData,
	)
	if kanjiIndex < 0 {
		t.Fatalf("card with notes has no kanji button")
	}
	askIndex := slices.Index(
		cardData,
		fmt.Sprintf(
			formatStudyAskLLM,
			sessionID,
			0,
		),
	)
	if askIndex < 0 || kanjiIndex > askIndex {
		t.Fatalf(
			"kanji button index %d, ask button index %d; want kanji above ask",
			kanjiIndex,
			askIndex,
		)
	}

	flow.HandleCallback(
		ctx,
		kanjiCallback(10),
	)
	notes := lastEditMessage(
		t,
		api,
	)
	for _, want := range []string{
		"<b>清</b> · 형성문자\n氵(물) + 青(セイ · 발음)",
		"이 단어에선: せい (음독)\n이 한자의 음독: セイ · ショウ",
		"<b>潔</b>\n이 단어에선: けつ (음독)\n이 한자의 음독: ケツ",
	} {
		if !strings.Contains(
			notes.Text,
			want,
		) {
			t.Errorf(
				"notes view missing %q:\n%s",
				want,
				notes.Text,
			)
		}
	}
	if got := allCallbackData(notes); len(got) != 1 || got[0] != fmt.Sprintf(
		formatStudyCard,
		sessionID,
		10,
	) {
		t.Errorf(
			"notes keyboard = %v, want only the back-to-card button",
			got,
		)
	}

	flow.HandleCallback(
		ctx,
		studyCallback(
			formatStudyNext,
			sessionID,
			0,
			userID,
		),
	)
	plainCard := lastEditMessage(
		t,
		api,
	)
	if slices.Contains(
		allCallbackData(plainCard),
		fmt.Sprintf(
			formatStudyKanji,
			sessionID,
			11,
		),
	) {
		t.Errorf("card without notes has a kanji button")
	}
	sent := len(api.sentMessages)
	flow.HandleCallback(
		ctx,
		kanjiCallback(11),
	)
	if len(api.sentMessages) != sent {
		t.Errorf("kanji callback for a card without notes edited the message")
	}
}

func allCallbackData(edit tgbotapi.EditMessageTextConfig) []string {
	var datas []string
	if edit.ReplyMarkup == nil {
		return datas
	}
	for _, row := range edit.ReplyMarkup.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData != nil {
				datas = append(
					datas,
					*button.CallbackData,
				)
			}
		}
	}
	return datas
}
