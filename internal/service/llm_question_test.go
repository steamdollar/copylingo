package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

type fakeTipCandidates struct {
	created []*model.TipCandidate
	err     error
}

func (f *fakeTipCandidates) CreateCandidate(
	_ context.Context,
	candidate *model.TipCandidate,
) error {
	f.created = append(
		f.created,
		candidate,
	)
	return f.err
}

func TestLLMQuestionServiceAnswer(t *testing.T) {
	user := model.User{ID: 7, Language: "ja", ProficiencyLevel: "N4"}
	answerWith := func(
		answer string,
		err error,
	) *mockLLMClient {
		return &mockLLMClient{answerFn: func(
			context.Context,
			string,
		) (string, error) {
			return answer, err
		}}
	}

	t.Run(
		"stores question and answer as candidate",
		func(t *testing.T) {
			tips := &fakeTipCandidates{}
			svc := NewLLMQuestionService(
				answerWith(
					"answer",
					nil,
				),
				tips,
				"test-model",
			)
			got, err := svc.Answer(
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
			c := tips.created[0]
			if c.UserID != 7 || c.Username != "learner" || c.Language != "ja" || c.ProficiencyLevel != "N4" ||
				c.Question != "question" || c.Answer != "answer" || c.SourceModel == nil || *c.SourceModel != "test-model" {
				t.Fatalf(
					"candidate = %+v",
					c,
				)
			}
		},
	)

	t.Run(
		"candidate failure still returns answer",
		func(t *testing.T) {
			svc := NewLLMQuestionService(
				answerWith(
					"answer",
					nil,
				),
				&fakeTipCandidates{err: errors.New("db down")},
				" ",
			)
			got, err := svc.Answer(
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
			svc := NewLLMQuestionService(
				answerWith(
					"",
					llmErr,
				),
				tips,
				"test-model",
			)
			if _, err := svc.Answer(
				context.Background(),
				user,
				"learner",
				"p",
				"q",
			); !errors.Is(
				err,
				llmErr,
			) {
				t.Fatalf(
					"error = %v, want %v",
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

// Without a tip-capable LLM client, top-up reports AI unavailability and
// never touches the repository.
func TestTipServiceTopUpWithoutGeneratorLLM(t *testing.T) {
	svc := NewTipService(
		nil,
		nil,
		"",
	)
	if err := svc.TopUpBucket(
		context.Background(),
		"ja",
		"N5",
	); !errors.Is(
		err,
		external.ErrAIConfigMissing,
	) {
		t.Fatalf(
			"error = %v, want ErrAIConfigMissing",
			err,
		)
	}
}
