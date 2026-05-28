package service

import (
	"context"
	"log"
	"math/rand"

	"github.com/lsj/copylingo/internal/model"
)

const maxPerCategory = 6

var defaultCategoryOrder = []model.QuestionCategory{
	model.CategoryKana,
	model.CategoryHandwriting,
	model.CategoryVocabulary,
	model.CategoryGrammar,
}

type questionFetcher interface {
	GetNewQuestions(ctx context.Context, language, level, category string, limit int) ([]model.Question, error)
	GetByID(ctx context.Context, id int) (*model.Question, error)
}

type sessionStore interface {
	CreateSession(ctx context.Context, s *model.Session) error
	GetByID(ctx context.Context, id int) (*model.Session, error)
	GetPendingSessions(ctx context.Context, userID int64) ([]model.Session, error)
	GetInProgressSessions(ctx context.Context, userID int64) ([]model.Session, error)
	ListInProgress(ctx context.Context) ([]model.Session, error)
	Start(ctx context.Context, id int) error
}

type sessionQuestionStore interface {
	CreateSessionQuestions(ctx context.Context, sqs []model.SessionQuestion) error
	GetBySession(ctx context.Context, sessionID int) ([]model.SessionQuestion, error)
}

// SessionBuilderService creates learning sessions with appropriate question mix.
type SessionBuilderService struct {
	questionRepo        questionFetcher
	sessionRepo         sessionStore
	sessionQuestionRepo sessionQuestionStore
	srs                 srsScheduler
}

func NewSessionBuilderService(
	questionRepo questionFetcher,
	sessionRepo sessionStore,
	sessionQuestionRepo sessionQuestionStore,
	srs srsScheduler,
) *SessionBuilderService {
	return &SessionBuilderService{
		questionRepo:        questionRepo,
		sessionRepo:         sessionRepo,
		sessionQuestionRepo: sessionQuestionRepo,
		srs:                 srs,
	}
}

// BuildMorningSession creates a morning session: 60% new + 40% review, total 15 questions.
func (s *SessionBuilderService) BuildMorningSession(ctx context.Context, userID int64, language, level string) (*model.Session, error) {
	const totalQuestions = 15
	const newQuestionCount = 9 // 60%
	const reviewCount = 6      // 40%

	return s.buildSession(ctx, userID, language, level, model.SessionMorning, totalQuestions, newQuestionCount, reviewCount)
}

// BuildEveningSession creates an evening session: 20% supplementary + 80% review, total 10 questions.
func (s *SessionBuilderService) BuildEveningSession(ctx context.Context, userID int64, language, level string) (*model.Session, error) {
	const totalQuestions = 10
	const newQuestionCount = 2 // 20% supplementary
	const reviewCount = 8      // 80%

	return s.buildSession(ctx, userID, language, level, model.SessionEvening, totalQuestions, newQuestionCount, reviewCount)
}

// BuildReviewSession creates an on-demand review session from SRS due items.
func (s *SessionBuilderService) BuildReviewSession(ctx context.Context, userID int64, limit int) (*model.Session, error) {
	return s.buildSession(ctx, userID, "", "", model.SessionReview, limit, 0, limit)
}

func (s *SessionBuilderService) buildSession(
	ctx context.Context,
	userID int64,
	language, level string,
	sessionType model.SessionType,
	totalQuestions, newCount, reviewCount int,
) (*model.Session, error) {
	var sessionQuestions []model.SessionQuestion
	order := 0

	// 1. Get review questions from SRS (due reviews)
	if reviewCount > 0 {
		reviews, err := s.srs.GetDueReviews(ctx, reviewCount)
		if err != nil {
			log.Printf("Error getting due reviews: %v", err)
		} else {
			for _, q := range reviews {
				sessionQuestions = append(sessionQuestions, model.SessionQuestion{
					QuestionID:    q.ID,
					QuestionOrder: order,
					IsReview:      true,
				})
				order++
			}
		}
	}

	// 2. Fill remaining with new questions (Random Slot Relay)
	remainingNew := totalQuestions - len(sessionQuestions)
	if remainingNew < newCount {
		remainingNew = newCount
	}

	if remainingNew > 0 && language != "" && level != "" {
		// Prepare categories for relay. The last empty category acts as a general fallback.
		categories := make([]string, 0, len(defaultCategoryOrder)+1)
		for _, cat := range defaultCategoryOrder {
			categories = append(categories, string(cat))
		}
		categories = append(categories, "") // Final fallback

		for i, cat := range categories {
			if remainingNew <= 0 {
				break
			}

			var alloc int
			if i == len(categories)-1 {
				// Final category gets all remaining slots
				alloc = remainingNew
			} else {
				// Random allocation with a per-category cap
				max := maxPerCategory
				if remainingNew < max {
					max = remainingNew
				}
				// rand.Intn(max+1) returns a value in [0, max]
				alloc = rand.Intn(max + 1)
			}

			if alloc > 0 {
				newQs, err := s.questionRepo.GetNewQuestions(ctx, language, level, cat, alloc)
				if err != nil {
					log.Printf("Error getting new questions for category %s: %v", cat, err)
					continue
				}

				for _, q := range newQs {
					sessionQuestions = append(sessionQuestions, model.SessionQuestion{
						QuestionID:    q.ID,
						QuestionOrder: order,
						IsReview:      false,
					})
					order++
				}
				// Deduct the number of questions actually fetched
				remainingNew -= len(newQs)
			}
		}
	}

	if len(sessionQuestions) == 0 {
		return nil, nil // No questions available
	}

	// Create session
	session := &model.Session{
		UserID:         userID,
		Type:           sessionType,
		Status:         model.SessionPending,
		TotalQuestions: len(sessionQuestions),
	}

	if err := s.sessionRepo.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	// Create session_questions entries
	for i := range sessionQuestions {
		sessionQuestions[i].SessionID = session.ID
	}
	if err := s.sessionQuestionRepo.CreateSessionQuestions(ctx, sessionQuestions); err != nil {
		return nil, err
	}

	return session, nil
}

// GetPendingSessions returns pending sessions for a user.
func (s *SessionBuilderService) GetPendingSessions(ctx context.Context, userID int64) ([]model.Session, error) {
	return s.sessionRepo.GetPendingSessions(ctx, userID)
}

// GetInProgressSessions returns in-progress sessions for a user.
func (s *SessionBuilderService) GetInProgressSessions(ctx context.Context, userID int64) ([]model.Session, error) {
	return s.sessionRepo.GetInProgressSessions(ctx, userID)
}

// GetAllInProgressSessions returns all in-progress sessions for all users.
func (s *SessionBuilderService) GetAllInProgressSessions(ctx context.Context) ([]model.Session, error) {
	return s.sessionRepo.ListInProgress(ctx)
}

// GetSession returns a session by ID.
func (s *SessionBuilderService) GetSession(ctx context.Context, sessionID int) (*model.Session, error) {
	return s.sessionRepo.GetByID(ctx, sessionID)
}

// StartSession marks a session as in_progress.
func (s *SessionBuilderService) StartSession(ctx context.Context, sessionID int) error {
	return s.sessionRepo.Start(ctx, sessionID)
}

// GetQuestion returns a question by ID.
func (s *SessionBuilderService) GetQuestion(ctx context.Context, questionID int) (*model.Question, error) {
	return s.questionRepo.GetByID(ctx, questionID)
}

// GetSessionQuestions returns all questions for a session.
func (s *SessionBuilderService) GetSessionQuestions(ctx context.Context, sessionID int) ([]model.SessionQuestion, error) {
	return s.sessionQuestionRepo.GetBySession(ctx, sessionID)
}
