package redisstore

import (
	"fmt"

	"github.com/lsj/copylingo/internal/model"
)

func NewQuizSessions(redisClient sessionRedis) *SessionStore[model.QuizActiveSessionState] {
	return newSessionStore[model.QuizActiveSessionState](
		redisClient,
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

func NewStudySessions(redisClient sessionRedis) *SessionStore[model.StudyActiveSessionState] {
	return newSessionStore[model.StudyActiveSessionState](
		redisClient,
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
