package service

import (
	"context"
	"log"
	"math/rand"
	"strings"

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

// sessionBuilderService creates learning sessions with appropriate question mix.
type sessionBuilderService struct {
	questionRepository        QuestionRepository
	sessionRepository         SessionRepository
	sessionQuestionRepository SessionQuestionRepository
}

func newSessionBuilderService(
	questionRepository QuestionRepository,
	sessionRepository SessionRepository,
	sessionQuestionRepository SessionQuestionRepository,
) *sessionBuilderService {
	return &sessionBuilderService{
		questionRepository:        questionRepository,
		sessionRepository:         sessionRepository,
		sessionQuestionRepository: sessionQuestionRepository,
	}
}

// BuildMorningSession creates a 17-question session, normally starting with 6 reviews.
func (service *sessionBuilderService) BuildMorningSession(
	ctx context.Context,
	userID int64,
	language,
	level string,
) (*model.Session, error) {
	const totalQuestions = 17
	const reviewCount = 6

	return service.buildSession(
		ctx,
		userID,
		language,
		level,
		model.SessionMorning,
		totalQuestions,
		reviewCount,
	)
}

// BuildEveningSession creates an evening session with vocabulary and listening reservations, total 17 questions.
// It matches the morning size so new quiz questions keep pace with daily Study (ADR-078).
func (service *sessionBuilderService) BuildEveningSession(
	ctx context.Context,
	userID int64,
	language,
	level string,
) (*model.Session, error) {
	const totalQuestions = 17
	const reviewCount = 6

	return service.buildSession(
		ctx,
		userID,
		language,
		level,
		model.SessionEvening,
		totalQuestions,
		reviewCount,
	)
}

// BuildReviewSession creates an on-demand review session from SRS due items.
func (service *sessionBuilderService) BuildReviewSession(
	ctx context.Context,
	userID int64,
	language,
	level string,
	limit int,
) (*model.Session, error) {
	return service.buildSession(
		ctx,
		userID,
		language,
		level,
		model.SessionReview,
		limit,
		limit,
	)
}

func (service *sessionBuilderService) buildSession(
	ctx context.Context,
	userID int64,
	language,
	level string,
	sessionType model.SessionType,
	totalQuestions,
	reviewCount int,
) (*model.Session, error) {
	var sessionQuestions []model.SessionQuestion
	currentLevels := []string{level}
	levels := sessionLevelsFor(
		language,
		level,
	)
	selectedIDs := make(
		map[int]bool,
		totalQuestions,
	)
	selectedMaintenanceMaterials := make(map[int]bool)
	excludeIDs := make(
		[]int,
		0,
		totalQuestions,
	)
	kanjiCount, readingCount := 0, 0
	currentCategories := make(map[model.QuestionCategory]int)

	appendQuestion := func(
		question model.Question,
		isReview bool,
	) bool {
		// Queries with different level/category scopes can nominate different
		// variants. Keep the maintenance cap across the entire assembled session.
		if question.IsMaintenanceCheck && question.MaterialID != nil &&
			selectedMaintenanceMaterials[*question.MaterialID] {
			return false
		}
		if len(sessionQuestions) >= totalQuestions || selectedIDs[question.ID] ||
			(isKanjiRecallQuestion(question) && kanjiCount >= maxKanjiRecallPerSession) ||
			(question.Category == model.CategoryReading && readingCount >= maxReadingPerSession) {
			return false
		}
		selectedIDs[question.ID] = true
		if question.IsMaintenanceCheck && question.MaterialID != nil {
			selectedMaintenanceMaterials[*question.MaterialID] = true
		}
		excludeIDs = append(
			excludeIDs,
			question.ID,
		)
		sessionQuestions = append(
			sessionQuestions,
			model.SessionQuestion{
				QuestionID: question.ID, QuestionOrder: len(sessionQuestions), IsReview: isReview,
			},
		)
		if isKanjiRecallQuestion(question) {
			kanjiCount++
		}
		if question.Category == model.CategoryReading {
			readingCount++
		}
		if isCurrentLevelQuestion(
			question,
			level,
		) {
			currentCategories[question.Category]++
		}
		return true
	}
	loadDue := func(
		limit int,
		categories ...model.QuestionCategory,
	) []model.Question {
		questions, err := service.questionRepository.GetDueReviews(
			ctx,
			userID,
			language,
			level,
			levels,
			limit,
			maxKanjiRecallPerSession,
			categories...,
		)
		if err != nil {
			log.Printf(
				"Error getting due reviews: %v",
				err,
			)
			return nil
		}
		return questions
	}

	if sessionType == model.SessionReview {
		for _, question := range loadDue(totalQuestions) {
			appendQuestion(
				question,
				true,
			)
		}
	} else if language != "" && level != "" {
		reservedVocabulary := divideRoundingUp(
			totalQuestions,
			minVocabularyRatioDenominator,
		)
		// Both reserved categories must fit alongside the new vocabulary floor.
		reviewCount = min(
			reviewCount,
			totalQuestions-reservedVocabulary-minListeningPerDailySession-maxReadingPerSession,
		)
		currentTarget := divideRoundingUp(
			totalQuestions*4,
			5,
		)

		// Query reserved due categories separately: a vocabulary backlog must not
		// hide current listening/reading beyond the general pool's LIMIT.
		reservedDueCount := 0
		for _, category := range []model.QuestionCategory{model.CategoryListening, model.CategoryReading} {
			for _, question := range loadDue(
				1,
				category,
			) {
				if isCurrentLevelQuestion(
					question,
					level,
				) && question.Category == category && appendQuestion(
					question,
					true,
				) {
					reservedDueCount++
					break
				}
			}
		}
		// Extra rows cover overlap with the two reserved queries. SQL applies
		// the reading/kanji caps before LIMIT, so a cap cannot hide other due rows.
		due := loadDue(totalQuestions + 2)
		appendDue := func(
			goal int,
			accept func(model.Question) bool,
		) {
			for _, question := range due {
				if len(sessionQuestions) >= goal {
					break
				}
				if accept(question) {
					appendQuestion(
						question,
						true,
					)
				}
			}
		}
		isCurrent := func(question model.Question) bool {
			return isCurrentLevelQuestion(
				question,
				level,
			)
		}
		appendDue(
			max(
				reviewCount,
				reservedDueCount,
			),
			isCurrent,
		)

		exhausted := make(map[string]bool)
		// fetchNew appends up to count new questions from scope and reports how
		// many it actually added.
		fetchNew := func(
			scope []string,
			category model.QuestionCategory,
			count int,
		) int {
			count = min(
				count,
				totalQuestions-len(sessionQuestions),
			)
			if category == model.CategoryReading {
				count = min(
					count,
					maxReadingPerSession-readingCount,
				)
			}
			key := strings.Join(
				scope,
				",",
			) + ":" + string(category)
			if count <= 0 || exhausted[key] {
				return 0
			}
			questions, err := service.questionRepository.GetNewQuestions(
				ctx,
				userID,
				language,
				scope,
				string(category),
				excludeIDs,
				count,
				maxKanjiRecallPerSession-kanjiCount,
			)
			if err != nil {
				log.Printf(
					"Error getting new questions for category %s: %v",
					category,
					err,
				)
				return 0
			}
			// Exclusions only grow during a build; do not query an exhausted
			// category again during current/adjacent shortage handling.
			exhausted[key] = len(questions) < count
			added := 0
			for _, question := range questions {
				if added >= count {
					break
				}
				if appendQuestion(
					question,
					false,
				) {
					added++
				}
			}
			return added
		}
		// reserveNew fills a reserved category from the current level first. Once
		// the current level has no new supply, the remaining slots borrow from the
		// harder neighbor, then the easier one, instead of waiting until every
		// current category runs dry (ADR-079).
		lowerLevel, upperLevel := neighborLevels(
			language,
			level,
		)
		reserveNew := func(
			category model.QuestionCategory,
			count int,
		) {
			added := fetchNew(
				currentLevels,
				category,
				count,
			)
			for _, neighbor := range []string{upperLevel, lowerLevel} {
				if neighbor != "" && added < count {
					added += fetchNew(
						[]string{neighbor},
						category,
						count-added,
					)
				}
			}
		}
		fetchNew(
			currentLevels,
			model.CategoryVocabulary,
			reservedVocabulary,
		)
		// Preserve the existing new-listening reservation even when a due
		// listening item was selected. Reading still has a one-item total cap.
		reserveNew(
			model.CategoryListening,
			minListeningPerDailySession,
		)
		if currentCategories[model.CategoryReading] == 0 {
			reserveNew(
				model.CategoryReading,
				1,
			)
		}

		fillNew := func(
			scope []string,
			goal int,
		) {
			// Retain the category relay, then exhaust each category explicitly.
			// A generic query can otherwise return only capped reading items and
			// incorrectly suggest that no current-level questions remain.
			for _, category := range defaultCategoryOrder {
				remaining := goal - len(sessionQuestions)
				if remaining <= 0 {
					return
				}
				fetchNew(
					scope,
					category,
					rand.Intn(min(
						maxPerCategory,
						remaining,
					)+1),
				)
			}
			for _, category := range defaultCategoryOrder {
				remaining := goal - len(sessionQuestions)
				if remaining <= 0 {
					return
				}
				fetchNew(
					scope,
					category,
					remaining,
				)
			}
		}

		// No adjacent questions enter before current candidates have had a
		// chance to meet the target. Due can exceed its usual budget when new
		// supply is scarce, rather than yielding those slots to another level.
		fillNew(
			currentLevels,
			currentTarget,
		)
		appendDue(
			currentTarget,
			isCurrent,
		)
		appendDue(
			totalQuestions,
			func(question model.Question) bool {
				return isLowerAdjacentQuestion(
					question,
					language,
					level,
				)
			},
		)
		if len(sessionQuestions) < totalQuestions {
			fillNew(
				currentLevels,
				totalQuestions,
			)
			appendDue(
				totalQuestions,
				isCurrent,
			)
		}
		if len(sessionQuestions) < totalQuestions && !sameLevelScope(
			currentLevels,
			levels,
		) {
			appendDue(
				totalQuestions,
				func(model.Question) bool { return true },
			)
			fillNew(
				levels,
				totalQuestions,
			)
		}
	}

	if len(sessionQuestions) == 0 {
		return nil, nil
	}
	session := &model.Session{
		UserID: userID, Type: sessionType, Mode: model.SessionModeQuiz,
		Status: model.SessionPending, TotalQuestions: len(sessionQuestions),
	}
	if err := service.sessionRepository.CreateSession(
		ctx,
		session,
	); err != nil {
		return nil, err
	}
	for i := range sessionQuestions {
		sessionQuestions[i].SessionID = session.ID
	}
	if err := service.sessionQuestionRepository.CreateSessionQuestions(
		ctx,
		sessionQuestions,
	); err != nil {
		return nil, err
	}
	return session, nil
}

func isKanjiRecallQuestion(question model.Question) bool {
	return question.Skill != nil && *question.Skill == model.SkillVocabularyKanjiRecall
}

func divideRoundingUp(
	dividend,
	divisor int,
) int {
	return (dividend + divisor - 1) / divisor
}

func isCurrentLevelQuestion(
	question model.Question,
	currentLevel string,
) bool {
	return question.ProficiencyLevel != "" && strings.EqualFold(
		question.ProficiencyLevel,
		currentLevel,
	)
}

func isLowerAdjacentQuestion(
	question model.Question,
	language,
	currentLevel string,
) bool {
	if question.ProficiencyLevel == "" {
		return false
	}
	scope := sessionLevelsFor(
		language,
		currentLevel,
	)
	currentIndex := -1
	questionIndex := -1
	for i, level := range scope {
		if strings.EqualFold(
			level,
			currentLevel,
		) {
			currentIndex = i
		}
		if strings.EqualFold(
			level,
			question.ProficiencyLevel,
		) {
			questionIndex = i
		}
	}
	return currentIndex >= 0 && questionIndex >= 0 && questionIndex < currentIndex
}

func sameLevelScope(
	current,
	scope []string,
) bool {
	if len(current) != len(scope) {
		return false
	}
	for i := range current {
		if !strings.EqualFold(
			current[i],
			scope[i],
		) {
			return false
		}
	}
	return true
}
