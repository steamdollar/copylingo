package service

import (
	"context"
	"fmt"

	"github.com/lsj/copylingo/internal/model"
)

// BuildForSlot creates the session a scheduled push slot calls for: Study
// slots use the fixed morning/evening study plan and Quiz slots the
// morning/evening question mix. nil means no content was available.
func (s *SessionService) BuildForSlot(
	ctx context.Context,
	user model.User,
	slot model.SessionSlot,
) (*model.Session, error) {
	switch slot {
	case model.SessionSlotMorningStudy:
		return s.BuildStudy(
			ctx,
			user,
			StudyProfileMorning,
			0,
		)
	case model.SessionSlotMorningQuiz:
		return s.BuildMorningQuiz(
			ctx,
			user,
		)
	case model.SessionSlotEveningStudy:
		return s.BuildStudy(
			ctx,
			user,
			StudyProfileEvening,
			0,
		)
	case model.SessionSlotEveningQuiz:
		return s.BuildEveningQuiz(
			ctx,
			user,
		)
	default:
		return nil, fmt.Errorf(
			"unsupported session slot: %s",
			slot,
		)
	}
}
