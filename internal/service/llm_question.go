package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/lsj/copylingo/internal/model"
)

// LearningQuestionLLM answers a free-form learning question.
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

// LLMQuestionService answers learner questions and keeps each Q&A as a tip
// candidate for curation. Prompt context (quiz/study card) is built by the caller.
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

// Answer asks the LLM with prompt and stores question/answer as a tip
// candidate. Candidate storage is best-effort: a failure is logged and the
// answer is still returned.
func (s *LLMQuestionService) Answer(
	ctx context.Context,
	user model.User,
	username,
	prompt,
	question string,
) (string, error) {
	answer, err := s.llm.AnswerLearningQuestion(
		ctx,
		prompt,
	)
	if err != nil {
		return "", err
	}

	var sourceModel *string
	if strings.TrimSpace(s.sourceModel) != "" {
		modelName := s.sourceModel
		sourceModel = &modelName
	}
	if err := s.tips.CreateCandidate(
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
