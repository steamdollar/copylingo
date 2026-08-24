package config

import "fmt"

// session status
type SessionStatus string

const (
	SessionStatusPending    SessionStatus = "pending"
	SessionStatusInProgress SessionStatus = "in_progress"
	SessionStatusCompleted  SessionStatus = "completed"
)

// Callback Data Prefixes
const (
	PrefixMenu     = "menu:"
	PrefixSession  = "session:"
	PrefixQuestion = "q:"
	PrefixStudy    = "study:"
)

// Callback Data Actions
const (
	// LLM 질의 대기 상태 취소
	ActionLLMCancel = "llm:cancel"
	// 메인 메뉴 화면 출력
	ActionMenuMain = "menu:main"
	// 일반 학습 세션 시작
	ActionMenuStudy = "menu:study"
	// 미루어둔 복습 세션 시작
	ActionMenuReview = "menu:review"
	// 메뉴 내 통계 보기 클릭시
	ActionMenuStats = "menu:stats"
	// 설정 메뉴(언어, 레벨 등) 열기
	ActionMenuSettings = "menu:settings"
)

type BotCommand string

// Bot Commands
const (
	// 봇 시작 및 환영 메시지
	CommandStart BotCommand = "start"
	// 메인 메뉴 표시
	CommandMenu BotCommand = "menu"
	// 상세 학습 통계 조회
	CommandStats BotCommand = "stats"
	// 현재 스트릭(연속 학습 일수) 확인
	CommandStreak BotCommand = "streak"
	// Study Material 세션 즉시 생성 및 발송
	CommandStudy BotCommand = "study"
	// LLM 질의 모드 활성화
	CommandLLM BotCommand = "llm"
	// 테스트용 세션 즉시 발송
	CommandTest BotCommand = "test"
	// 도움말 및 명령어 안내
	CommandHelp BotCommand = "help"
	// 현재 입력 취소 및 대기 상태 종료
	CommandExit BotCommand = "exit"
)

// LLMAllowedTelegramUserIDs are the Telegram users allowed to use /llm.
var LLMAllowedTelegramUserIDs = [...]int64{
	2006481393,
}

// Callback Data Formats (for Sprintf)
const (
	FormatSessionStart   = "session:%d:start"
	FormatSessionFinish  = "session:%d:finish"
	FormatQuestionAnswer = "q:%d:%d:%d"
	FormatQuestionNext   = "q:%d:next:%d"
	FormatQuestionAskLLM = "q:%d:ask:%d"
	// Word-order callbacks carry the session/question identity and the action;
	// a select action also carries the original option index. Keeping this
	// compact leaves ample room under Telegram's 64-byte callback limit.
	FormatWordOrderSelect = "q:%d:wo:%d:a:%d"
	FormatWordOrderUndo   = "q:%d:wo:%d:u"
	FormatWordOrderReset  = "q:%d:wo:%d:r"
	FormatWordOrderSubmit = "q:%d:wo:%d:s"
	FormatStudyStart      = "study:%d:start"
	FormatStudyNext       = "study:%d:next:%d"
	FormatStudyPrev       = "study:%d:prev:%d"
	FormatStudyFinish     = "study:%d:finish:%d"
	FormatStudyAskLLM     = "study:%d:ask:%d"
)

type RedisKeyFormat string

func (k RedisKeyFormat) Format(args ...any) string {
	return fmt.Sprintf(string(k), args...)
}

// Redis Key Patterns
const (
	// SessionQuestionStartRedisKey stores when the currently displayed question was shown.
	// Value: Unix milliseconds. Used for per-question timing/observability.
	SessionQuestionStartRedisKey RedisKeyFormat = "session:%d:question_start"

	// ActiveSessionWorkingSetRedisKey stores the full in-progress session working set.
	// Value: JSON-encoded model.ActiveSessionState containing session metadata,
	// ordered session_questions, question copies, progress, current index, and timestamps.
	ActiveSessionWorkingSetRedisKey RedisKeyFormat = "session:%d:working_set"

	// StudySessionWorkingSetRedisKey stores the full in-progress study session working set.
	// Value: JSON-encoded model.StudyActiveSessionState containing session metadata,
	// ordered session_materials, material copies, progress, current index, and timestamps.
	StudySessionWorkingSetRedisKey RedisKeyFormat = "study_session:%d:working_set"

	// UserActiveQuestionRedisKey tracks the text-answer question currently waiting for a chat reply.
	// Value: "session_id:question_index". Used by fill-blank/subjective text input handling.
	UserActiveQuestionRedisKey RedisKeyFormat = "user:%d:active_question"

	// UserLLMPendingRedisKey tracks that the next plain-text message should be routed to LLM question mode.
	// Value: "1" for a plain /llm question, "q:{session_id}:{question_id}" for a quiz question, or
	// "study:{session_id}:{material_order}" for a study material. Used by one-shot contextual LLM questions.
	UserLLMPendingRedisKey RedisKeyFormat = "user:%d:llm_pending"

	// HandwritingMessageRedisKey stores the Telegram message that contains a handwriting Mini App button.
	// Value: "chat_id:message_id". Used to remove stale inline buttons after Mini App submission.
	HandwritingMessageRedisKey RedisKeyFormat = "handwriting:msg:%d:%d"

	// WordOrderDraftRedisKey stores a JSON array of original option indices for
	// one active session question. Its TTL matches the active session working
	// set; the draft never lives inside ActiveSessionState.
	WordOrderDraftRedisKey RedisKeyFormat = "session:%d:word_order:%d:draft"
)

// Mini App routes
const (
	PathHandwritingMiniApp = "/miniapp/handwriting"
	PathHandwritingSubmit  = "/api/miniapp/handwriting/submit"
	PathMiniAppTips        = "/api/miniapp/tips"
)
