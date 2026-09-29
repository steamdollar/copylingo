package service

import (
	"context"
	"fmt"

	"github.com/lsj/copylingo/internal/external"
)

type llmService struct {
	client external.LLMClient
}

func newLLMService(client external.LLMClient) *llmService {
	return &llmService{client: client}
}

func (s *llmService) GradeAnswer(
	ctx context.Context,
	questionPrompt,
	correctAnswer,
	userAnswer string,
) (external.GradeResult, error) {
	if s == nil || s.client == nil {
		return external.GradeResult{}, external.ErrAIConfigMissing
	}
	return s.client.GradeAnswer(
		ctx,
		questionPrompt,
		correctAnswer,
		userAnswer,
	)
}

func (s *llmService) GradeHandwriting(
	ctx context.Context,
	questionPrompt,
	correctAnswer string,
	pngImage []byte,
) (external.GradeResult, error) {
	if s == nil || s.client == nil {
		return external.GradeResult{}, external.ErrAIConfigMissing
	}
	return s.client.GradeHandwriting(
		ctx,
		questionPrompt,
		correctAnswer,
		pngImage,
	)
}

func (s *llmService) AnswerLearningQuestion(
	ctx context.Context,
	question string,
) (string, error) {
	if s == nil || s.client == nil {
		return "", external.ErrAIConfigMissing
	}
	answer, err := s.client.AnswerLearningQuestion(
		ctx,
		question,
	)
	if err != nil {
		return "", fmt.Errorf(
			"answer llm learning question: %w",
			err,
		)
	}
	return answer, nil
}
