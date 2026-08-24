package service

import (
	"context"
	"log"
	"math/rand"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

const (
	maxPerCategory                = 6
	maxKanjiRecallPerSession      = 3
	maxReadingPerSession          = 1
	minVocabularyRatioDenominator = 3
	minListeningPerDailySession   = 1
)

var defaultCategoryOrder = []model.QuestionCategory{
	model.CategoryKana,
	model.CategoryHandwriting,
	model.CategoryVocabulary,
	model.CategoryGrammar,
	// Listening joins the relay; GetNewQuestions only returns listening items
	// whose audio is already generated (audio_path IS NOT NULL), so audio-less
	// questions are never scheduled (ADR-031/032).
	model.CategoryListening,
	// Reading joins the relay; GetNewQuestions only returns reading items whose
	// passage material the user already studied, and every session caps reading
	// at one question — passages take longer to solve (ADR-036).
	model.CategoryReading,
}

type questionFetcher interface {
	GetNewQuestions(
		ctx context.Context,
		userID int64,
		language, level, category string,
		excludeIDs []int,
		limit, kanjiRecallLimit int,
	) ([]model.Question, error)
	GetByID(ctx context.Context, id int) (*model.Question, error)
}

type sessionStore interface {
	CreateSession(ctx context.Context, s *model.Session) error
	GetByID(ctx context.Context, id int) (*model.Session, error)
	GetSessionsByStatus(ctx context.Context, userID int64, status config.SessionStatus) ([]model.Session, error)
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

// BuildMorningSession creates a morning session with up to 6 reviews, total 17 questions.
func (s *SessionBuilderService) BuildMorningSession(
	ctx context.Context,
	userID int64,
	language, level string,
) (*model.Session, error) {
	const totalQuestions = 17
	const reviewCount = 6

	return s.buildSession(ctx, userID, language, level, model.SessionMorning, totalQuestions, reviewCount)
}

// BuildEveningSession creates an evening session with vocabulary and listening reservations, total 12 questions.
func (s *SessionBuilderService) BuildEveningSession(
	ctx context.Context,
	userID int64,
	language, level string,
) (*model.Session, error) {
	const totalQuestions = 12
	const reviewCount = 8 // Reduced to 7 when reserving 4 vocabulary and 1 listening slot.

	return s.buildSession(ctx, userID, language, level, model.SessionEvening, totalQuestions, reviewCount)
}

// BuildReviewSession creates an on-demand review session from SRS due items.
func (s *SessionBuilderService) BuildReviewSession(
	ctx context.Context,
	userID int64,
	language, level string,
	limit int,
) (*model.Session, error) {
	return s.buildSession(ctx, userID, language, level, model.SessionReview, limit, limit)
}

func (s *SessionBuilderService) buildSession(
	ctx context.Context,
	userID int64,
	language, level string,
	sessionType model.SessionType,
	totalQuestions, reviewCount int,
) (*model.Session, error) {
	var sessionQuestions []model.SessionQuestion
	selectedQuestionIDs := make(map[int]struct{}, totalQuestions)
	excludeIDs := make([]int, 0, totalQuestions)
	order := 0
	kanjiRecallCount := 0
	reservedVocabularyCount := 0
	reservedListeningCount := 0
	if sessionType != model.SessionReview && language != "" && level != "" {
		reservedVocabularyCount = divideRoundingUp(totalQuestions, minVocabularyRatioDenominator)
		reservedListeningCount = minListeningPerDailySession
		if maxReviewCount := totalQuestions - reservedVocabularyCount - reservedListeningCount; reviewCount > maxReviewCount {
			reviewCount = maxReviewCount
		}
	}

	readingCount := 0
	appendQuestion := func(question model.Question, isReview bool) bool {
		if _, exists := selectedQuestionIDs[question.ID]; exists {
			return false
		}
		if isKanjiRecallQuestion(question) && kanjiRecallCount >= maxKanjiRecallPerSession {
			return false
		}
		if question.Category == model.CategoryReading && readingCount >= maxReadingPerSession {
			return false
		}
		selectedQuestionIDs[question.ID] = struct{}{}
		excludeIDs = append(excludeIDs, question.ID)
		sessionQuestions = append(sessionQuestions, model.SessionQuestion{
			QuestionID:    question.ID,
			QuestionOrder: order,
			IsReview:      isReview,
		})
		if isKanjiRecallQuestion(question) {
			kanjiRecallCount++
		}
		if question.Category == model.CategoryReading {
			readingCount++
		}
		order++
		return true
	}

	// 1. Get review questions from SRS (due reviews)
	if reviewCount > 0 {
		reviews, err := s.srs.GetDueReviews(
			ctx,
			userID,
			language,
			level,
			reviewCount,
			maxKanjiRecallPerSession,
		)
		if err != nil {
			log.Printf("Error getting due reviews: %v", err)
		} else {
			for _, q := range reviews {
				appendQuestion(q, true)
			}
		}
	}

	// 2. Reserve at least one-third of daily session slots for new vocabulary.
	// If vocabulary inventory is short, the relay below fills the remaining slots.
	if reservedVocabularyCount > 0 {
		newQs, err := s.questionRepo.GetNewQuestions(
			ctx,
			userID,
			language,
			level,
			string(model.CategoryVocabulary),
			excludeIDs,
			reservedVocabularyCount,
			maxKanjiRecallPerSession-kanjiRecallCount,
		)
		if err != nil {
			log.Printf("Error getting reserved vocabulary questions: %v", err)
		} else {
			for _, q := range newQs {
				appendQuestion(q, false)
			}
		}
	}

	// 3. Reserve one new listening slot in each daily session. If no audio-ready
	// unseen listening question exists, the relay below fills the unused slot.
	if reservedListeningCount > 0 {
		newQs, err := s.questionRepo.GetNewQuestions(
			ctx,
			userID,
			language,
			level,
			string(model.CategoryListening),
			excludeIDs,
			reservedListeningCount,
			maxKanjiRecallPerSession-kanjiRecallCount,
		)
		if err != nil {
			log.Printf("Error getting reserved listening questions: %v", err)
		} else {
			for _, q := range newQs {
				appendQuestion(q, false)
			}
		}
	}

	// 4. Fill remaining with new questions (Random Slot Relay)
	remainingNew := totalQuestions - len(sessionQuestions)

	if sessionType != model.SessionReview && remainingNew > 0 && language != "" && level != "" {
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
			// Never fetch more reading questions than the per-session cap admits.
			if cat == string(model.CategoryReading) {
				if remaining := maxReadingPerSession - readingCount; alloc > remaining {
					alloc = remaining
				}
			}

			if alloc > 0 {
				newQs, err := s.questionRepo.GetNewQuestions(
					ctx,
					userID,
					language,
					level,
					cat,
					excludeIDs,
					alloc,
					maxKanjiRecallPerSession-kanjiRecallCount,
				)
				if err != nil {
					log.Printf("Error getting new questions for category %s: %v", cat, err)
					continue
				}

				added := 0
				for _, q := range newQs {
					if appendQuestion(q, false) {
						added++
					}
				}
				// Deduct the number of questions actually fetched
				remainingNew -= added
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
		Mode:           model.SessionModeQuiz,
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

func isKanjiRecallQuestion(question model.Question) bool {
	return question.Skill != nil && *question.Skill == model.SkillVocabKanjiRecall
}

func divideRoundingUp(dividend, divisor int) int {
	return (dividend + divisor - 1) / divisor
}

func (s *SessionBuilderService) GetSessionsByStatus(
	ctx context.Context,
	userID int64,
	status config.SessionStatus,
) ([]model.Session, error) {
	return s.sessionRepo.GetSessionsByStatus(ctx, userID, status)
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
func (s *SessionBuilderService) GetSessionQuestions(
	ctx context.Context,
	sessionID int,
) ([]model.SessionQuestion, error) {
	return s.sessionQuestionRepo.GetBySession(ctx, sessionID)
}
