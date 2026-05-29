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
)

// Callback Data Actions
const (
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
	// 테스트용 세션 즉시 발송
	CommandTest BotCommand = "test"
	// 도움말 및 명령어 안내
	CommandHelp BotCommand = "help"
	// 현재 입력 취소 및 대기 상태 종료
	CommandExit BotCommand = "exit"
)

// Callback Data Formats (for Sprintf)
const (
	FormatSessionStart   = "session:%d:start"
	FormatSessionFinish  = "session:%d:finish"
	FormatQuestionAnswer = "q:%d:%d:%d"
	FormatQuestionNext   = "q:%d:next:%d"
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

	// UserActiveQuestionRedisKey tracks the text-answer question currently waiting for a chat reply.
	// Value: "session_id:question_index". Used by fill-blank/subjective text input handling.
	UserActiveQuestionRedisKey RedisKeyFormat = "user:%d:active_question"

	// HandwritingMessageRedisKey stores the Telegram message that contains a handwriting Mini App button.
	// Value: "chat_id:message_id". Used to remove stale inline buttons after Mini App submission.
	HandwritingMessageRedisKey RedisKeyFormat = "handwriting:msg:%d:%d"
)

// Mini App routes
const (
	PathHandwritingMiniApp = "/miniapp/handwriting"
	PathHandwritingSubmit  = "/api/miniapp/handwriting/submit"
	PathMiniAppTips        = "/api/miniapp/tips"
)
