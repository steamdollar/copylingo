package service

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/repository"
)

const (
	DefaultStudySessionMaterialCount = 20
	MaxStudySessionMaterialCount     = 50
)

// StudySessionProfile selects the fixed quota policy used by an automated
// study push.
type StudySessionProfile string

const (
	StudyProfileMorning StudySessionProfile = "morning"
	StudyProfileEvening StudySessionProfile = "evening"
)

var morningStudySessionPlan = model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
	{Category: model.MaterialCategoryVocabulary, NewCount: 8, ReviewCount: 7},
	{Category: model.MaterialCategoryGrammar, NewCount: 1, ReviewCount: 3},
	{Category: model.MaterialCategoryReading, NewCount: 1, ReviewCount: 0},
}}

var eveningStudySessionPlan = model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
	{Category: model.MaterialCategoryVocabulary, NewCount: 4, ReviewCount: 14},
	{Category: model.MaterialCategoryGrammar, NewCount: 1, ReviewCount: 3},
	{Category: model.MaterialCategoryReading, NewCount: 0, ReviewCount: 2},
}}

// scaleMorningStudySessionPlan scales the morning category weights (15:4:1)
// to a requested limit. Reading is capped at two slots; any excess is
// redistributed between vocabulary and grammar using their 15:4 weights.
func scaleMorningStudySessionPlan(limit int) model.StudySessionPlan {
	// Above 40 materials, keep reading at two and distribute the rest by the
	// vocabulary-to-grammar ratio.
	var categoryTotals [3]int
	if limit <= 40 {
		allocation := largestRemainderStudyAllocation(
			[]int{15, 4, 1},
			limit,
		)
		categoryTotals = [3]int{allocation[0], allocation[1], allocation[2]}
	} else {
		allocation := largestRemainderStudyAllocation(
			[]int{15, 4},
			limit-2,
		)
		categoryTotals = [3]int{allocation[0], allocation[1], 2}
	}
	vocabularyNew := scaleStudyNewCount(
		categoryTotals[0],
		8,
		15,
	)
	grammarNew := scaleStudyNewCount(
		categoryTotals[1],
		1,
		4,
	)
	readingNew := scaleStudyNewCount(
		categoryTotals[2],
		1,
		1,
	)
	return model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{
			Category:    model.MaterialCategoryVocabulary,
			NewCount:    vocabularyNew,
			ReviewCount: categoryTotals[0] - vocabularyNew,
		},
		{Category: model.MaterialCategoryGrammar, NewCount: grammarNew, ReviewCount: categoryTotals[1] - grammarNew},
		{Category: model.MaterialCategoryReading, NewCount: readingNew, ReviewCount: categoryTotals[2] - readingNew},
	}}
}

// largestRemainderStudyAllocation uses integer arithmetic so ties are stable
// in the declared category order.
func largestRemainderStudyAllocation(
	weights []int,
	total int,
) []int {
	if total <= 0 || len(weights) == 0 {
		return make(
			[]int,
			len(weights),
		)
	}
	weightTotal := 0
	for _, weight := range weights {
		weightTotal += weight
	}
	if weightTotal <= 0 {
		return make(
			[]int,
			len(weights),
		)
	}

	allocation := make(
		[]int,
		len(weights),
	)
	remainders := make(
		[]int,
		len(weights),
	)
	allocated := 0
	for i, weight := range weights {
		numerator := total * weight
		allocation[i] = numerator / weightTotal
		remainders[i] = numerator % weightTotal
		allocated += allocation[i]
	}
	for remaining := total - allocated; remaining > 0; remaining-- {
		best := 0
		for i := 1; i < len(weights); i++ {
			if remainders[i] > remainders[best] {
				best = i
			}
		}
		allocation[best]++
		remainders[best] = -1
	}
	return allocation
}

func scaleStudyNewCount(
	total,
	newWeight,
	baseTotal int,
) int {
	if total <= 0 || newWeight <= 0 || baseTotal <= 0 {
		return 0
	}
	newCount := (total*newWeight + baseTotal/2) / baseTotal
	if newCount == 0 {
		newCount = 1
	}
	if newCount > total {
		return total
	}
	return newCount
}

type studyMaterialStore interface {
	GetMaterialsByPlan(
		ctx context.Context,
		userID int64,
		language,
		level string,
		levels []string,
		plan model.StudySessionPlan,
	) ([]model.Material, error)
}

type studySessionStore interface {
	CreateSessionInTx(
		ctx context.Context,
		tx *sqlx.Tx,
		session *model.Session,
	) (int, error)
	CreateSessionMaterialsInTx(
		ctx context.Context,
		tx *sqlx.Tx,
		sessionID int,
		materialIDs []int,
	) error
}

// StudySessionService creates material-based study sessions.
type StudySessionService struct {
	materialRepo studyMaterialStore
	sessionRepo  studySessionStore
	db           *sqlx.DB
}

func NewStudySessionService(
	materialRepo studyMaterialStore,
	sessionRepo studySessionStore,
	db *sqlx.DB,
) *StudySessionService {
	return &StudySessionService{
		materialRepo: materialRepo,
		sessionRepo:  sessionRepo,
		db:           db,
	}
}

// BuildStudySession selects the fixed morning/evening plan, or scales the
// morning plan when limit is positive, then creates the session in the DB.
func (s *StudySessionService) BuildStudySession(
	ctx context.Context,
	userID int64,
	language,
	level string,
	profile StudySessionProfile,
	limit int,
	// deprecate 하던가 리팩토링 필요
) (*model.Session, error) {
	// A positive limit scales only the morning plan; evening keeps its fixed plan.
	if limit < 0 || limit > MaxStudySessionMaterialCount {
		return nil, fmt.Errorf(
			"build study session invalid limit user_id=%d limit=%d",
			userID,
			limit,
		)
	}

	// retrieve profile > check the type of profile > fix plan
	var plan model.StudySessionPlan
	switch profile {
	case StudyProfileMorning:
		if limit == 0 {
			// default
			plan = morningStudySessionPlan
		} else {
			// TODO: 이 부분 정책, 코드 관련 정리 필요
			plan = scaleMorningStudySessionPlan(limit)
		}
	case StudyProfileEvening:
		plan = eveningStudySessionPlan
	default:
		return nil, fmt.Errorf(
			"build study session invalid profile user_id=%d profile=%s",
			userID,
			profile,
		)
	}

	// validate plan
	if plan.TotalMaterialCount() <= 0 ||
		plan.TotalMaterialCount() > MaxStudySessionMaterialCount {
		return nil, fmt.Errorf(
			"build study session invalid plan user_id=%d total=%d",
			userID,
			plan.TotalMaterialCount(),
		)
	}

	// get materials
	materials, err := s.materialRepo.GetMaterialsByPlan(
		ctx,
		userID,
		language,
		level,
		sessionLevelsFor(
			language,
			level,
		),
		plan,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"build study session fetch materials user_id=%d language=%s level=%s: %w",
			userID,
			language,
			level,
			err,
		)
	}
	if len(materials) == 0 {
		return nil, nil
	}

	session := &model.Session{
		UserID:         userID,
		Type:           model.SessionStudy,
		Mode:           model.SessionModeStudy,
		Status:         model.SessionPending,
		TotalQuestions: len(materials),
	}
	materialIDs := make(
		[]int,
		len(materials),
	)
	for index, material := range materials {
		materialIDs[index] = material.ID
	}
	// The service owns the boundary: both rows commit together after the ordered links are saved.
	var sessionID int
	if err := repository.WithinTx(
		ctx,
		s.db,
		func(tx *sqlx.Tx) error {
			var err error
			sessionID, err = s.sessionRepo.CreateSessionInTx(
				ctx,
				tx,
				session,
			)
			if err != nil {
				return err
			}
			return s.sessionRepo.CreateSessionMaterialsInTx(
				ctx,
				tx,
				sessionID,
				materialIDs,
			)
		},
	); err != nil {
		return nil, fmt.Errorf(
			"build study session create user_id=%d: %w",
			userID,
			err,
		)
	}
	session.ID = sessionID

	return session, nil
}
