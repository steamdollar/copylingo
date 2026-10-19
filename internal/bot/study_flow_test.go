package bot

import (
	"context"
	"encoding/json"
	"errors"
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

func (s *botStudyMaterialStore) GetMaterialsByPlan(
	ctx context.Context,
	userID int64,
	language,
	level string,
	levels []string,
	plan model.StudySessionPlan,
) ([]model.Material, error) {
	return nil, nil
}

// botStudySessionStore drives Study creation, start and completion; the
// embedded nil SessionRepo makes any other sessions-table call panic.
type botStudySessionStore struct {
	service.SessionRepo
	session    *model.Session
	started    []int
	completed  []int
	correctCnt []int
}

func (s *botStudySessionStore) CreateSessionInTx(
	ctx context.Context,
	tx *sqlx.Tx,
	session *model.Session,
) (int, error) {
	return 0, nil
}

func (s *botStudySessionStore) CreateSessionMaterialsInTx(
	ctx context.Context,
	tx *sqlx.Tx,
	sessionID int,
	materialIDs []int,
) error {
	return nil
}

func (s *botStudySessionStore) GetByID(
	ctx context.Context,
	id int,
) (*model.Session, error) {
	return s.session, nil
}

func (s *botStudySessionStore) Start(
	ctx context.Context,
	id int,
) error {
	s.started = append(
		s.started,
		id,
	)
	if s.session != nil {
		s.session.Status = model.SessionInProgress
	}
	return nil
}

func (s *botStudySessionStore) Complete(
	ctx context.Context,
	id int,
	correctCount int,
) error {
	s.completed = append(
		s.completed,
		id,
	)
	s.correctCnt = append(
		s.correctCnt,
		correctCount,
	)
	if s.session != nil {
		s.session.Status = model.SessionCompleted
	}
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
		Session:   *r.session,
		Items:     r.items,
		UpdatedAt: timeNowForTest(),
	}, nil
}

func (r *botStudyActiveRepo) FlushStudyActiveSession(
	ctx context.Context,
	state *model.StudyActiveSessionState,
) error {
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
	activeRepo := &botStudyActiveRepo{
		session: sessionStore.session,
		items:   items,
	}
	stateStores := newTestInteractionStores()
	session := newTestSessionService(
		stateStores,
		service.SessionDeps{
			MaterialRepo:           &botStudyMaterialStore{},
			SessionRepo:            sessionStore,
			StudyActiveSessionRepo: activeRepo,
			DB:                     botTestDB,
		},
	)
	flow := newTestStudyFlow(
		api,
		nil,
		StudyFlowDeps{
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
	if activeRepo.flushedState == nil {
		t.Fatal("expected study active session to flush on finish")
	}
	if activeRepo.flushedState.Items[0].SessionMaterial.StudiedAt == nil ||
		activeRepo.flushedState.Items[1].SessionMaterial.StudiedAt == nil {
		t.Fatalf(
			"expected all materials studied before flush: %+v",
			activeRepo.flushedState.Items,
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
	activeRepo := &botStudyActiveRepo{
		session: sessionStore.session,
		items:   items,
	}
	stateStores := newTestInteractionStores()
	session := newTestSessionService(
		stateStores,
		service.SessionDeps{
			SessionRepo:            sessionStore,
			StudyActiveSessionRepo: activeRepo,
		},
	)
	flow := newTestStudyFlow(
		api,
		nil,
		StudyFlowDeps{
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

	// 마지막 카드(2/2)에서 이전으로 돌아가면 첫 카드가 다시 보인다.
	flow.HandleCallback(
		ctx,
		studyCallback(
			formatStudyPrev,
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
	// 첫 카드에는 이전 버튼이 없다.
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

	// 이미 studied인 카드에서 다시 앞으로 이동한 뒤 완료까지 정상 진행된다.
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
	activeRepo := &botStudyActiveRepo{
		session: sessionStore.session,
		items:   items,
	}
	stateStores := newTestInteractionStores()
	flow := newTestStudyFlow(
		api,
		nil,
		StudyFlowDeps{
			Session: newTestSessionService(
				stateStores,
				service.SessionDeps{
					MaterialRepo:           &botStudyMaterialStore{},
					SessionRepo:            sessionStore,
					StudyActiveSessionRepo: activeRepo,
					DB:                     botTestDB,
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
	activeRepo := &botStudyActiveRepo{
		session: sessionStore.session,
		items:   items,
	}
	stateStores := newTestInteractionStores()
	flow := newTestStudyFlow(
		api,
		nil,
		StudyFlowDeps{
			Session: newTestSessionService(
				stateStores,
				service.SessionDeps{
					MaterialRepo:           &botStudyMaterialStore{},
					SessionRepo:            sessionStore,
					StudyActiveSessionRepo: activeRepo,
					DB:                     botTestDB,
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
	for _, tc := range []struct {
		name    string
		showAsk bool
		wantAsk bool
	}{
		{name: "owner sees ask button", showAsk: true, wantAsk: true},
		{name: "non-owner has no ask button", showAsk: false, wantAsk: false},
	} {
		t.Run(
			tc.name,
			func(t *testing.T) {
				keyboard := studyMaterialKeyboard(
					99,
					3,
					false,
					false,
					tc.showAsk,
				)
				found := false
				for _, row := range keyboard.InlineKeyboard {
					for _, button := range row {
						if button.CallbackData != nil && *button.CallbackData == askData {
							found = true
						}
					}
				}
				if found != tc.wantAsk {
					t.Errorf(
						"ask button present = %t, want %t",
						found,
						tc.wantAsk,
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
	case formatStudyPrev:
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

func intString(v int) string {
	return strconv.Itoa(v)
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
	// Every Study card carries the navigation row plus the study settings row;
	// only the navigation row (first) is inspected.
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

// seiketsuPayload is a vocabulary payload whose 清 carries a kanji note.
const seiketsuPayload = `{"kana":"せいけつ","kanji":"清潔","meaning_ko":"청결함","part_of_speech":"noun",` +
	`"kanji_notes":[{"char":"清","type":"phono_semantic",` +
	`"parts":[{"part":"氵","role":"meaning","gloss_ko":"물"},{"part":"青","role":"sound","on":"セイ"}],` +
	`"explanation_ko":"물(氵)처럼 맑다는 뜻에 青이 음 セイ를 더한다.",` +
	`"series":[{"char":"晴","on":"セイ"},{"char":"精","on":"セイ"}]}]}`

// botKanjiSeriesWords returns canned studied words and records the lookup.
type botKanjiSeriesWords struct {
	words []model.StudiedSeriesWord
	err   error
	notes []model.KanjiNote
}

func (k *botKanjiSeriesWords) StudiedSeriesWords(
	ctx context.Context,
	userID int64,
	material model.Material,
	notes []model.KanjiNote,
) ([]model.StudiedSeriesWord, error) {
	k.notes = notes
	return k.words, k.err
}

// TestVocabularyKanjiNotesDecidesButton pins the button rule: only a
// vocabulary payload with at least one note has notes.
func TestVocabularyKanjiNotesDecidesButton(t *testing.T) {
	t.Parallel()

	base := `"kana":"せいけつ","kanji":"清潔","meaning_ko":"청결함","part_of_speech":"noun"`
	for _, tc := range []struct {
		name     string
		category model.MaterialCategory
		payload  string
		want     int
	}{
		{name: "notes", category: model.MaterialCategoryVocabulary, payload: seiketsuPayload, want: 1},
		{name: "absent", category: model.MaterialCategoryVocabulary, payload: "{" + base + "}", want: 0},
		{name: "null", category: model.MaterialCategoryVocabulary, payload: "{" + base + `,"kanji_notes":null}`, want: 0},
		{name: "empty", category: model.MaterialCategoryVocabulary, payload: "{" + base + `,"kanji_notes":[]}`, want: 0},
		{name: "not vocabulary", category: model.MaterialCategoryGrammar, payload: seiketsuPayload, want: 0},
	} {
		t.Run(
			tc.name,
			func(t *testing.T) {
				got := vocabularyKanjiNotes(model.Material{
					Category: tc.category,
					Payload:  json.RawMessage(tc.payload),
				})
				if len(got) != tc.want {
					t.Errorf(
						"notes = %d, want %d",
						len(got),
						tc.want,
					)
				}
			},
		)
	}
}

// TestRenderStudyMaterialIgnoresKanjiNotes keeps the card text identical with
// or without notes; the notes only appear behind the button.
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

func TestRenderKanjiNotes(t *testing.T) {
	t.Parallel()

	vocab := vocabularyStudyPayload{
		Kana:  "まつ",
		Kanji: "待つ",
		KanjiNotes: []model.KanjiNote{
			{
				Char: "待",
				Type: model.KanjiNotePhonoSemantic,
				Parts: []model.KanjiPart{
					{Part: "彳", Role: model.KanjiPartMeaning, GlossKo: "길을 감"},
					{Part: "寺", Role: model.KanjiPartSound},
				},
				ExplanationKo: "寺(ジ)는 원래 발음 요소지만 일본어 음은 タイ다.",
			},
			{
				Char: "読",
				Type: model.KanjiNotePhonoSemantic,
				Parts: []model.KanjiPart{
					{Part: "言", Role: model.KanjiPartMeaning, GlossKo: "말"},
					{Part: "売", Role: model.KanjiPartForm, GlossKo: "간략화된 <모양>"},
				},
				ExplanationKo: "구자체 讀.",
			},
		},
	}
	studied := []model.StudiedSeriesWord{
		{SoundPart: "寺", Word: "時計"},
		{SoundPart: "寺", Word: "持つ"},
		{SoundPart: "青", Word: "晴れ"},
	}
	got := renderKanjiNotes(
		vocab,
		studied,
	)
	for _, want := range []string{
		"<b>待つ</b> (まつ)",
		"<b>待</b> · 형성문자\n彳(길을 감) + 寺(발음)\n寺(ジ)는 원래 발음 요소지만 일본어 음은 タイ다.\n寺 계열로 배운 단어: 時計 · 持つ",
		"言(말) + 売(간략화된 &lt;모양&gt;)",
	} {
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
	if strings.Contains(
		got,
		"晴れ",
	) {
		t.Errorf(
			"studied word of an unrelated sound part was shown:\n%s",
			got,
		)
	}
}

// TestStudyFlowKanjiNotes walks card → notes → card for a word with notes and
// checks that a word without notes gets neither the button nor a notes view.
func TestStudyFlowKanjiNotes(t *testing.T) {
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
		service.SessionDeps{
			MaterialRepo: &botStudyMaterialStore{},
			SessionRepo:  sessionStore,
			StudyActiveSessionRepo: &botStudyActiveRepo{
				session: sessionStore.session,
				items:   items,
			},
			DB: botTestDB,
		},
	)
	series := &botKanjiSeriesWords{
		words: []model.StudiedSeriesWord{
			{SoundPart: "青", Word: "晴れ"},
			{SoundPart: "青", Word: "精神"},
		},
	}
	flow := newTestStudyFlow(
		api,
		nil,
		StudyFlowDeps{
			Session:   session,
			KanjiNote: series,
		},
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
	if !slices.Contains(
		allCallbackData(lastEditMessage(
			t,
			api,
		)),
		kanjiData,
	) {
		t.Fatalf("card with notes has no kanji button")
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
		"<b>清</b> · 형성문자",
		"氵(물) + 青(セイ · 발음)",
		"같은 발음 요소: 晴(セイ) · 精(セイ)",
		"青 계열로 배운 단어: 晴れ · 精神",
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
	if len(series.notes) != 1 || series.notes[0].Char != "清" {
		t.Errorf(
			"series lookup notes = %+v, want the 清 note",
			series.notes,
		)
	}

	// A failed series lookup still shows the notes, without the studied line.
	series.err = errors.New("db down")
	flow.HandleCallback(
		ctx,
		kanjiCallback(10),
	)
	degraded := lastEditMessage(
		t,
		api,
	).Text
	if !strings.Contains(
		degraded,
		"氵(물) + 青(セイ · 발음)",
	) || strings.Contains(
		degraded,
		"배운 단어",
	) {
		t.Errorf(
			"degraded notes view = %q",
			degraded,
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
