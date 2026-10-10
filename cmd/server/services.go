package main

import (
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/bootstrap"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/redisstore"
	"github.com/lsj/copylingo/internal/repository"
	"github.com/lsj/copylingo/internal/service"
)

// services holds the Tier1 services cmd/server hands to bot, scheduler and
// Mini App. It stays inside cmd/server: each consumer receives only the
// fields it calls (ADR-059 §8.3).
type services struct {
	user               *service.UserService
	session            *service.SessionService
	materialPreference *service.MaterialPreferenceService
	analyzer           *service.AnalyzerService
	tip                *service.TipService
	llmQuestion        *service.LLMQuestionService
	audio              *service.AudioService // nil without an LLM API key
}

// newServices creates the repositories and external clients, then the Tier1
// services over them.
func newServices(
	cfg *config.Config,
	db *sqlx.DB,
	rdb *redis.Client,
) services {
	repos := repository.NewRepositories(db)

	// One LLM client serves Quiz grading, learner questions and tip generation.
	llm := external.NewLLMClient(external.LLMOptions{
		APIKey:  cfg.LLM.APIKey,
		BaseURL: cfg.LLM.BaseURL,
		Model:   cfg.LLM.Model,
	})
	tip := service.NewTipService(
		repos.Tip,
		llm,
		cfg.LLM.Model,
	)

	// Listening audio is available whenever the shared API key is configured.
	// With no key, audio stays nil; SessionFlow and the scheduler skip it.
	var audio *service.AudioService
	if cfg.LLM.APIKey != "" {
		audio = bootstrap.NewAudioService(
			cfg,
			repos.Question,
		)
	}

	// Only the storage implementations know Redis commands and serialized keys.
	session := service.NewSessionService(service.SessionDeps{
		QuestionRepo:           repos.Question,
		SessionRepo:            repos.Session,
		SessionQuestionRepo:    repos.SessionQuestion,
		QuizActiveSessionRepo:  repos.QuizActiveSession,
		StudyActiveSessionRepo: repos.StudyActiveSession,
		MaterialRepo:           repos.Material,
		DB:                     db,
		UserRepo:               repos.User,
		Stores: service.SessionStores{
			Quiz:  redisstore.NewQuizSessions(rdb),
			Study: redisstore.NewStudySessions(rdb),
		},
		LLM: llm,
	})

	return services{
		user:               service.NewUserService(repos.User),
		session:            session,
		materialPreference: service.NewMaterialPreferenceService(repos.MaterialPreference),
		analyzer: service.NewAnalyzerService(
			repos.User,
			repos.SessionQuestion,
		),
		tip: tip,
		llmQuestion: service.NewLLMQuestionService(
			llm,
			tip,
			cfg.LLM.Model,
		),
		audio: audio,
	}
}
