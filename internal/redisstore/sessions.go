package redisstore

import (
	"fmt"

	"github.com/lsj/copylingo/internal/model"
)

// NewQuizSessions selects the Quiz state type and key for the shared store.
// The caller must provide an initialized Redis client.
func NewQuizSessions(rdb sessionRedis) *SessionStore[model.QuizActiveSessionState] {
	return newSessionStore[model.QuizActiveSessionState](
		rdb,
		func(sessionID int) string {
			return fmt.Sprintf(
				"session:%d:working_set",
				sessionID,
			)
		},
		"quiz",
		func(
			state *model.QuizActiveSessionState,
			sessionID int,
		) bool {
			return state.Version == model.QuizActiveSessionStateVersion && state.Session.ID == sessionID
		},
	)
}

// NewStudySessions selects the Study state type and key for the shared store.
// The caller must provide an initialized Redis client.
func NewStudySessions(rdb sessionRedis) *SessionStore[model.StudyActiveSessionState] {
	return newSessionStore[model.StudyActiveSessionState](
		rdb,
		func(sessionID int) string {
			return fmt.Sprintf(
				"study_session:%d:working_set",
				sessionID,
			)
		},
		"study",
		func(
			state *model.StudyActiveSessionState,
			sessionID int,
		) bool {
			return state.Session.ID == sessionID
		},
	)
}
