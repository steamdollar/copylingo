package service

import (
	"context"
	"log"
	"math/rand"
	"strings"

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
		language string,
		levels []string,
		category string,
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

// BuildMorningSession creates a 17-question session, normally starting with 6 reviews.
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
	const reviewCount = 8 // Clamped below to leave room for vocabulary, listening, and reading.

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
	currentLevels := []string{level}
	levels := sessionLevelsFor(language, level)
	selectedIDs := make(map[int]bool, totalQuestions)
	excludeIDs := make([]int, 0, totalQuestions)
	kanjiCount, readingCount := 0, 0
	currentCategories := make(map[model.QuestionCategory]int)

	appendQuestion := func(q model.Question, isReview bool) bool {
		if len(sessionQuestions) >= totalQuestions || selectedIDs[q.ID] ||
			(isKanjiRecallQuestion(q) && kanjiCount >= maxKanjiRecallPerSession) ||
			(q.Category == model.CategoryReading && readingCount >= maxReadingPerSession) {
			return false
		}
		selectedIDs[q.ID] = true
		excludeIDs = append(excludeIDs, q.ID)
		sessionQuestions = append(sessionQuestions, model.SessionQuestion{
			QuestionID: q.ID, QuestionOrder: len(sessionQuestions), IsReview: isReview,
		})
		if isKanjiRecallQuestion(q) {
			kanjiCount++
		}
		if q.Category == model.CategoryReading {
			readingCount++
		}
		if isCurrentLevelQuestion(q, level) {
			currentCategories[q.Category]++
		}
		return true
	}
	loadDue := func(limit int, categories ...model.QuestionCategory) []model.Question {
		questions, err := s.srs.GetDueReviews(ctx, userID, language, level,
			limit, maxKanjiRecallPerSession, categories...)
		if err != nil {
			log.Printf("Error getting due reviews: %v", err)
			return nil
		}
		return questions
	}

	if sessionType == model.SessionReview {
		for _, q := range loadDue(totalQuestions) {
			appendQuestion(q, true)
		}
	} else if language != "" && level != "" {
		reservedVocabulary := divideRoundingUp(totalQuestions, minVocabularyRatioDenominator)
		// Both reserved categories must fit alongside the new vocabulary floor.
		reviewCount = min(
			reviewCount,
			totalQuestions-reservedVocabulary-minListeningPerDailySession-maxReadingPerSession,
		)
		currentTarget := divideRoundingUp(totalQuestions*4, 5)

		// Query reserved due categories separately: a vocabulary backlog must not
		// hide current listening/reading beyond the general pool's LIMIT.
		reservedDueCount := 0
		for _, category := range []model.QuestionCategory{model.CategoryListening, model.CategoryReading} {
			for _, q := range loadDue(1, category) {
				if isCurrentLevelQuestion(q, level) && q.Category == category && appendQuestion(q, true) {
					reservedDueCount++
					break
				}
			}
		}
		// Extra rows cover overlap with the two reserved queries. SQL applies
		// the reading/kanji caps before LIMIT, so a cap cannot hide other due rows.
		due := loadDue(totalQuestions + 2)
		appendDue := func(goal int, accept func(model.Question) bool) {
			for _, q := range due {
				if len(sessionQuestions) >= goal {
					break
				}
				if accept(q) {
					appendQuestion(q, true)
				}
			}
		}
		isCurrent := func(q model.Question) bool { return isCurrentLevelQuestion(q, level) }
		appendDue(max(reviewCount, reservedDueCount), isCurrent)

		exhausted := make(map[string]bool)
		fetchNew := func(scope []string, category model.QuestionCategory, count int) {
			count = min(count, totalQuestions-len(sessionQuestions))
			if category == model.CategoryReading {
				count = min(count, maxReadingPerSession-readingCount)
			}
			key := strings.Join(scope, ",") + ":" + string(category)
			if count <= 0 || exhausted[key] {
				return
			}
			questions, err := s.questionRepo.GetNewQuestions(ctx, userID, language, scope,
				string(category), excludeIDs, count, maxKanjiRecallPerSession-kanjiCount)
			if err != nil {
				log.Printf("Error getting new questions for category %s: %v", category, err)
				return
			}
			// Exclusions only grow during a build; do not query an exhausted
			// category again during current/adjacent shortage handling.
			exhausted[key] = len(questions) < count
			added := 0
			for _, q := range questions {
				if added >= count {
					break
				}
				if appendQuestion(q, false) {
					added++
				}
			}
		}
		fetchNew(currentLevels, model.CategoryVocabulary, reservedVocabulary)
		// Preserve the existing new-listening reservation even when a due
		// listening item was selected. Reading still has a one-item total cap.
		fetchNew(currentLevels, model.CategoryListening, minListeningPerDailySession)
		if currentCategories[model.CategoryReading] == 0 {
			fetchNew(currentLevels, model.CategoryReading, 1)
		}

		fillNew := func(scope []string, goal int) {
			// Retain the category relay, then exhaust each category explicitly.
			// A generic query can otherwise return only capped reading items and
			// incorrectly suggest that no current-level questions remain.
			for _, category := range defaultCategoryOrder {
				remaining := goal - len(sessionQuestions)
				if remaining <= 0 {
					return
				}
				fetchNew(scope, category, rand.Intn(min(maxPerCategory, remaining)+1))
			}
			for _, category := range defaultCategoryOrder {
				remaining := goal - len(sessionQuestions)
				if remaining <= 0 {
					return
				}
				fetchNew(scope, category, remaining)
			}
		}

		// No adjacent questions enter before current candidates have had a
		// chance to meet the target. Due can exceed its usual budget when new
		// supply is scarce, rather than yielding those slots to another level.
		fillNew(currentLevels, currentTarget)
		appendDue(currentTarget, isCurrent)
		appendDue(totalQuestions, func(q model.Question) bool {
			return isLowerAdjacentQuestion(q, language, level)
		})
		if len(sessionQuestions) < totalQuestions {
			fillNew(currentLevels, totalQuestions)
			appendDue(totalQuestions, isCurrent)
		}
		if len(sessionQuestions) < totalQuestions && !sameLevelScope(currentLevels, levels) {
			appendDue(totalQuestions, func(model.Question) bool { return true })
			fillNew(levels, totalQuestions)
		}
	}

	if len(sessionQuestions) == 0 {
		return nil, nil
	}
	session := &model.Session{
		UserID: userID, Type: sessionType, Mode: model.SessionModeQuiz,
		Status: model.SessionPending, TotalQuestions: len(sessionQuestions),
	}
	if err := s.sessionRepo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
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

func isCurrentLevelQuestion(question model.Question, currentLevel string) bool {
	return question.ProficiencyLevel != "" && strings.EqualFold(question.ProficiencyLevel, currentLevel)
}

func isLowerAdjacentQuestion(question model.Question, language, currentLevel string) bool {
	if question.ProficiencyLevel == "" {
		return false
	}
	scope := sessionLevelsFor(language, currentLevel)
	currentIndex := -1
	questionIndex := -1
	for i, level := range scope {
		if strings.EqualFold(level, currentLevel) {
			currentIndex = i
		}
		if strings.EqualFold(level, question.ProficiencyLevel) {
			questionIndex = i
		}
	}
	return currentIndex >= 0 && questionIndex >= 0 && questionIndex < currentIndex
}

func sameLevelScope(current, scope []string) bool {
	if len(current) != len(scope) {
		return false
	}
	for i := range current {
		if !strings.EqualFold(current[i], scope[i]) {
			return false
		}
	}
	return true
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
