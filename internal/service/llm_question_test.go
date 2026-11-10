package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type fakeTipCandidates struct {
	created []*model.TipCandidate
	err     error
}

func (fake *fakeTipCandidates) CreateCandidate(
	_ context.Context,
	candidate *model.TipCandidate,
) error {
	fake.created = append(
		fake.created,
		candidate,
	)
	return fake.err
}

type fakeLearningQuestionLLM struct {
	answer string
	err    error
}

func (fake *fakeLearningQuestionLLM) AnswerLearningQuestion(
	context.Context,
	string,
) (string, error) {
	return fake.answer, fake.err
}

func TestLLMQuestionServiceAnswer(t *testing.T) {
	user := model.User{ID: 7, Language: "ja", ProficiencyLevel: "N4"}
	answerWith := func(
		answer string,
		err error,
	) *fakeLearningQuestionLLM {
		return &fakeLearningQuestionLLM{answer: answer, err: err}
	}

	t.Run(
		"stores question and answer as candidate",
		func(t *testing.T) {
			tips := &fakeTipCandidates{}
			llmQuestionService := NewLLMQuestionService(
				answerWith(
					"answer",
					nil,
				),
				tips,
				"test-model",
			)
			got, err := llmQuestionService.Answer(
				context.Background(),
				user,
				"learner",
				"context + question",
				"question",
			)
			if err != nil || got != "answer" {
				t.Fatalf(
					"Answer = %q, %v",
					got,
					err,
				)
			}
			if len(tips.created) != 1 {
				t.Fatalf(
					"candidates = %d, want 1",
					len(tips.created),
				)
			}
			candidate := tips.created[0]
			if candidate.UserID != 7 || candidate.Username != "learner" || candidate.Language != "ja" ||
				candidate.ProficiencyLevel != "N4" || candidate.Question != "question" || candidate.Answer != "answer" ||
				candidate.SourceModel == nil ||
				*candidate.SourceModel != "test-model" {
				t.Fatalf(
					"candidate = %+v",
					candidate,
				)
			}
		},
	)

	t.Run(
		"candidate failure still returns answer",
		func(t *testing.T) {
			llmQuestionService := NewLLMQuestionService(
				answerWith(
					"answer",
					nil,
				),
				&fakeTipCandidates{err: errors.New("db down")},
				" ",
			)
			got, err := llmQuestionService.Answer(
				context.Background(),
				user,
				"learner",
				"p",
				"q",
			)
			if err != nil || got != "answer" {
				t.Fatalf(
					"Answer = %q, %v; candidate storage is best-effort",
					got,
					err,
				)
			}
		},
	)

	t.Run(
		"LLM failure stores nothing",
		func(t *testing.T) {
			tips := &fakeTipCandidates{}
			llmErr := errors.New("provider failed")
			llmQuestionService := NewLLMQuestionService(
				answerWith(
					"",
					llmErr,
				),
				tips,
				"test-model",
			)
			if _, err := llmQuestionService.Answer(
				context.Background(),
				user,
				"learner",
				"p",
				"q",
			); !errors.Is(
				err,
				llmErr,
			) || !strings.Contains(
				err.Error(),
				"answer llm learning question: provider failed",
			) {
				t.Fatalf(
					"error = %v, want %v wrapped with the answer step",
					err,
					llmErr,
				)
			}
			if len(tips.created) != 0 {
				t.Fatal("no candidate may be stored without an answer")
			}
		},
	)
}
