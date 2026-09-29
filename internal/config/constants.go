package config

// SessionStatus is shared by session retrieval across bot, service, and repository.
type SessionStatus string

const (
	SessionStatusPending    SessionStatus = "pending"
	SessionStatusInProgress SessionStatus = "in_progress"
	SessionStatusCompleted  SessionStatus = "completed"
)

// Mini App routes are shared by the HTTP handler and the bot's Web App button.
const (
	PathHandwritingMiniApp = "/miniapp/handwriting"
	PathHandwritingSubmit  = "/api/miniapp/handwriting/submit"
	PathMiniAppTips        = "/api/miniapp/tips"
)
