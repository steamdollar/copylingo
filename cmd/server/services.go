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
	llmQuestion        *service.LLMQuestionService
	audio              *service.AudioService
}

// newServices creates the repositories and external clients, then the Tier1
// services over them.
func newServices(
	appConfig *config.Config,
	db *sqlx.DB,
	redisClient *redis.Client,
) services {
	repositories := repository.NewRepositories(db)

	// One LLM client serves Quiz grading and learner questions.
	llm := external.NewLLMClient(external.LLMOptions{
		APIKey:  appConfig.LLM.APIKey,
		BaseURL: appConfig.LLM.BaseURL,
		Model:   appConfig.LLM.Model,
	})

	// Only the storage implementations know Redis commands and serialized keys.
	session := service.NewSessionService(service.SessionDependencies{
		QuestionRepository:           repositories.Question,
		SessionRepository:            repositories.Session,
		SessionQuestionRepository:    repositories.SessionQuestion,
		QuizActiveSessionRepository:  repositories.QuizActiveSession,
		StudyActiveSessionRepository: repositories.StudyActiveSession,
		MaterialRepository:           repositories.Material,
		DB:                           db,
		UserRepository:               repositories.User,
		Stores: service.SessionStores{
			Quiz:  redisstore.NewQuizSessions(redisClient),
			Study: redisstore.NewStudySessions(redisClient),
		},
		LLM: llm,
	})

	return services{
		user:               service.NewUserService(repositories.User),
		session:            session,
		materialPreference: service.NewMaterialPreferenceService(repositories.MaterialPreference),
		analyzer: service.NewAnalyzerService(
			repositories.User,
			repositories.SessionQuestion,
			repositories.Material,
		),
		llmQuestion: service.NewLLMQuestionService(
			llm,
			repositories.Tip,
			appConfig.LLM.Model,
		),
		// Serves pre-generated listening clips; generation runs in
		// cmd/admin/generate_listening_audio (ADR-082).
		audio: service.NewAudioService(
			repositories.Question,
			bootstrap.NewAudioStore(appConfig.Storage),
		),
	}
}
