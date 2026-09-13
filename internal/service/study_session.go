package service

import (
	"context"
	"fmt"

	"github.com/lsj/copylingo/internal/model"
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

func studySessionPlanForProfile(profile StudySessionProfile) (model.StudySessionPlan, bool) {
	switch profile {
	case StudyProfileMorning:
		return morningStudySessionPlan, true
	case StudyProfileEvening:
		return eveningStudySessionPlan, true
	default:
		return model.StudySessionPlan{}, false
	}
}

// scaleMorningStudySessionPlan scales the morning category weights (15:4:1)
// to a requested limit. Reading is capped at two slots; any excess is
// redistributed between vocabulary and grammar using their 15:4 weights.
func scaleMorningStudySessionPlan(limit int) model.StudySessionPlan {
	categoryTotals := scaleStudyCategoryTotals(limit)
	vocabularyNew := scaleStudyNewCount(categoryTotals[0], 8, 15)
	grammarNew := scaleStudyNewCount(categoryTotals[1], 1, 4)
	readingNew := scaleStudyNewCount(categoryTotals[2], 1, 1)
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

func scaleStudyCategoryTotals(limit int) [3]int {
	if limit <= 40 {
		allocation := largestRemainderStudyAllocation([]int{15, 4, 1}, limit)
		return [3]int{allocation[0], allocation[1], allocation[2]}
	}

	vg := largestRemainderStudyAllocation([]int{15, 4}, limit-2)
	return [3]int{vg[0], vg[1], 2}
}

// largestRemainderStudyAllocation uses integer arithmetic so ties are stable
// in the declared category order.
func largestRemainderStudyAllocation(weights []int, total int) []int {
	if total <= 0 || len(weights) == 0 {
		return make([]int, len(weights))
	}
	weightTotal := 0
	for _, weight := range weights {
		weightTotal += weight
	}
	if weightTotal <= 0 {
		return make([]int, len(weights))
	}

	allocation := make([]int, len(weights))
	remainders := make([]int, len(weights))
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

func scaleStudyNewCount(total, newWeight, baseTotal int) int {
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
	GetForStudySession(
		ctx context.Context,
		userID int64,
		language, level string,
		levels []string,
		plan model.StudySessionPlan,
	) ([]model.Material, error)
}

type studySessionStore interface {
	CreateSession(ctx context.Context, s *model.Session) error
}

type studySessionMaterialStore interface {
	CreateSessionMaterials(ctx context.Context, sms []model.SessionMaterial) error
}

// StudySessionService creates material-based study sessions.
type StudySessionService struct {
	materialRepo        studyMaterialStore
	sessionRepo         studySessionStore
	sessionMaterialRepo studySessionMaterialStore
}

func NewStudySessionService(
	materialRepo studyMaterialStore,
	sessionRepo studySessionStore,
	sessionMaterialRepo studySessionMaterialStore,
) *StudySessionService {
	return &StudySessionService{
		materialRepo:        materialRepo,
		sessionRepo:         sessionRepo,
		sessionMaterialRepo: sessionMaterialRepo,
	}
}

func (s *StudySessionService) BuildStudySession(
	ctx context.Context,
	userID int64,
	language, level string,
) (*model.Session, error) {
	return s.BuildStudySessionWithProfile(ctx, userID, language, level, StudyProfileMorning)
}

// BuildStudySessionWithProfile creates a study session using a named
// automated-push policy.
func (s *StudySessionService) BuildStudySessionWithProfile(
	ctx context.Context,
	userID int64,
	language, level string,
	profile StudySessionProfile,
) (*model.Session, error) {
	plan, ok := studySessionPlanForProfile(profile)
	if !ok {
		return nil, fmt.Errorf("build study session invalid profile user_id=%d profile=%s", userID, profile)
	}
	return s.buildStudySessionWithPlan(ctx, userID, language, level, plan)
}

func (s *StudySessionService) BuildStudySessionWithLimit(
	ctx context.Context,
	userID int64,
	language, level string,
	limit int,
) (*model.Session, error) {
	if limit <= 0 || limit > MaxStudySessionMaterialCount {
		return nil, fmt.Errorf("build study session invalid limit user_id=%d limit=%d", userID, limit)
	}
	return s.buildStudySessionWithPlan(ctx, userID, language, level, scaleMorningStudySessionPlan(limit))
}

func (s *StudySessionService) buildStudySessionWithPlan(
	ctx context.Context,
	userID int64,
	language, level string,
	plan model.StudySessionPlan,
) (*model.Session, error) {
	if plan.TotalMaterialCount() <= 0 || plan.TotalMaterialCount() > MaxStudySessionMaterialCount {
		return nil, fmt.Errorf(
			"build study session invalid plan user_id=%d total=%d",
			userID,
			plan.TotalMaterialCount(),
		)
	}

	materials, err := s.materialRepo.GetForStudySession(
		ctx,
		userID,
		language,
		level,
		sessionLevelsFor(language, level),
		plan,
	)
	if err != nil {
		return nil, fmt.Errorf("build study session fetch materials user_id=%d language=%s level=%s: %w",
			userID, language, level, err)
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
	if err := s.sessionRepo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("build study session create user_id=%d: %w", userID, err)
	}

	sessionMaterials := make([]model.SessionMaterial, 0, len(materials))
	for i, material := range materials {
		sessionMaterials = append(sessionMaterials, model.SessionMaterial{
			SessionID:     session.ID,
			MaterialID:    material.ID,
			MaterialOrder: i,
		})
	}
	if err := s.sessionMaterialRepo.CreateSessionMaterials(ctx, sessionMaterials); err != nil {
		return nil, fmt.Errorf("build study session create materials session_id=%d: %w", session.ID, err)
	}

	return session, nil
}
