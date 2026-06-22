package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type mockLLMClient struct {
	answerFn func(ctx context.Context, question string) (string, error)
}

func (m *mockLLMClient) GradeAnswer(
	ctx context.Context,
	questionPrompt, correctAnswer, userAnswer string,
) (bool, string, error) {
	return true, "", nil
}

func (m *mockLLMClient) GradeHandwriting(
	ctx context.Context,
	questionPrompt, correctAnswer string,
	pngImage []byte,
) (bool, string, error) {
	return true, "", nil
}

func (m *mockLLMClient) AnswerLearningQuestion(ctx context.Context, question string) (string, error) {
	return m.answerFn(ctx, question)
}

func TestLLMServiceAnswerLearningQuestion(t *testing.T) {
	svc := NewLLMService(&mockLLMClient{
		answerFn: func(ctx context.Context, question string) (string, error) {
			if question != "honoo가 뭐야?" {
				t.Fatalf("question = %q", question)
			}
			return "불꽃입니다.", nil
		},
	})

	answer, err := svc.AnswerLearningQuestion(context.Background(), "honoo가 뭐야?")
	if err != nil {
		t.Fatalf("AnswerLearningQuestion error = %v", err)
	}
	if answer != "불꽃입니다." {
		t.Fatalf("answer = %q", answer)
	}
}

func TestLLMServiceAnswerLearningQuestionWrapsError(t *testing.T) {
	svc := NewLLMService(&mockLLMClient{
		answerFn: func(ctx context.Context, question string) (string, error) {
			return "", errors.New("provider failed")
		},
	})

	_, err := svc.AnswerLearningQuestion(context.Background(), "honoo가 뭐야?")
	if err == nil || !strings.Contains(err.Error(), "answer llm learning question: provider failed") {
		t.Fatalf("err = %v", err)
	}
}
