package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/lsj/copylingo/internal/model"
)

type LearningQuestionLLM interface {
	AnswerLearningQuestion(
		ctx context.Context,
		question string,
	) (string, error)
}

type tipCandidateCreator interface {
	CreateCandidate(
		ctx context.Context,
		candidate *model.TipCandidate,
	) error
}

// LLMQuestionService keeps each Q&A as a tip candidate; the caller builds the prompt context.
type LLMQuestionService struct {
	llm         LearningQuestionLLM
	tips        tipCandidateCreator
	sourceModel string
}

func NewLLMQuestionService(
	llm LearningQuestionLLM,
	tips tipCandidateCreator,
	sourceModel string,
) *LLMQuestionService {
	return &LLMQuestionService{
		llm:         llm,
		tips:        tips,
		sourceModel: sourceModel,
	}
}

// Candidate storage is best-effort: a failure is logged and the answer is still returned.
func (service *LLMQuestionService) Answer(
	ctx context.Context,
	user model.User,
	username,
	prompt,
	question string,
) (string, error) {
	answer, err := service.llm.AnswerLearningQuestion(
		ctx,
		prompt,
	)
	if err != nil {
		return "", fmt.Errorf(
			"answer llm learning question: %w",
			err,
		)
	}

	var sourceModel *string
	if strings.TrimSpace(service.sourceModel) != "" {
		modelName := service.sourceModel
		sourceModel = &modelName
	}
	if err := service.tips.CreateCandidate(
		ctx,
		&model.TipCandidate{
			UserID:           user.ID,
			Username:         username,
			Language:         user.Language,
			ProficiencyLevel: user.ProficiencyLevel,
			Question:         question,
			Answer:           answer,
			SourceModel:      sourceModel,
		},
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to create tip candidate",
			"event",
			"llm_question.tip_candidate_create_failed",
			"source",
			"service.llm_question",
			"user_id",
			user.ID,
			"language",
			user.Language,
			"level",
			user.ProficiencyLevel,
			"error",
			err,
		)
	}
	return answer, nil
}
