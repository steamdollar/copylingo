package service

import (
	"context"
	"fmt"

	"github.com/lsj/copylingo/internal/external"
)

type LLMService struct {
	client external.LLMClient
}

func NewLLMService(client external.LLMClient) *LLMService {
	return &LLMService{client: client}
}

func (s *LLMService) GradeAnswer(
	ctx context.Context,
	questionPrompt, correctAnswer, userAnswer string,
) (bool, string, error) {
	if s == nil || s.client == nil {
		return false, "", external.ErrAIConfigMissing
	}
	return s.client.GradeAnswer(ctx, questionPrompt, correctAnswer, userAnswer)
}

func (s *LLMService) GradeHandwriting(
	ctx context.Context,
	questionPrompt, correctAnswer string,
	pngImage []byte,
) (bool, string, error) {
	if s == nil || s.client == nil {
		return false, "", external.ErrAIConfigMissing
	}
	return s.client.GradeHandwriting(ctx, questionPrompt, correctAnswer, pngImage)
}

func (s *LLMService) AnswerLearningQuestion(ctx context.Context, question string) (string, error) {
	if s == nil || s.client == nil {
		return "", external.ErrAIConfigMissing
	}
	answer, err := s.client.AnswerLearningQuestion(ctx, question)
	if err != nil {
		return "", fmt.Errorf("answer llm learning question: %w", err)
	}
	return answer, nil
}
