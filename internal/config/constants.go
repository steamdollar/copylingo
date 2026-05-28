package config

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

// Bot Commands
const (
	// 봇 시작 및 환영 메시지
	CommandStart = "start"
	// 메인 메뉴 표시
	CommandMenu = "menu"
	// 상세 학습 통계 조회
	CommandStats = "stats"
	// 현재 스트릭(연속 학습 일수) 확인
	CommandStreak = "streak"
	// 테스트용 세션 즉시 발송
	CommandTest = "test"
	// 도움말 및 명령어 안내
	CommandHelp = "help"
	// 현재 입력 취소 및 대기 상태 종료
	CommandExit = "exit"
)

// Callback Data Formats (for Sprintf)
const (
	FormatSessionStart   = "session:%d:start"
	FormatSessionFinish  = "session:%d:finish"
	FormatQuestionAnswer = "q:%d:%d:%d"
	FormatQuestionNext   = "q:%d:next:%d"
)

// Redis Key Patterns
const (
	KeySessionQuestionStart string = "session:%d:question_start"
	KeyUserActiveQuestion   string = "user:%d:active_question"
	KeyHandwritingMessage   string = "handwriting:msg:%d:%d" // session_id, question_id
)

// Mini App routes
const (
	PathHandwritingMiniApp = "/miniapp/handwriting"
	PathHandwritingSubmit  = "/api/miniapp/handwriting/submit"
	PathMiniAppTips        = "/api/miniapp/tips"
)
