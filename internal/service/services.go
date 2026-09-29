package service

import (
	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/repository"
)

// Services holds all service instances.
type Services struct {
	Content            *ContentService
	User               *UserService
	Session            *SessionService
	MaterialPreference *MaterialPreferenceService
	Analyzer           *AnalyzerService
	Tip                *TipService
	LLMQuestion        *LLMQuestionService
	Audio              *AudioService
}

// NewServices creates all services with the given dependencies.
func NewServices(
	repos *repository.Repositories,
	db *sqlx.DB,
	cfg *config.Config,
	stores SessionStores,
) *Services {
	// One LLM client serves Quiz grading, learner questions and tip generation.
	llmClient := external.NewLLMClient(cfg)
	llm := newLLMService(llmClient)

	// GenerateTips lives on the concrete *DefaultLLMClient (not the LLMClient
	// interface). Without it, pass a true nil (not a typed nil) so tip top-up
	// reports ErrAIConfigMissing instead of calling a nil client.
	var tipLLM tipGeneratorLLM
	if concrete, ok := llmClient.(*external.DefaultLLMClient); ok {
		tipLLM = concrete
	}
	tip := NewTipService(
		repos.Tip,
		tipLLM,
		cfg.LLM.Model,
	)

	// Listening audio is available whenever the shared API key is configured.
	// With no key, Audio stays nil; the scheduler and bot tolerate that.
	var audioService *AudioService
	if cfg.LLM.APIKey != "" {
		audioService = NewAudioService(
			repos.Question,
			external.NewTTSClient(cfg),
			external.NewS3AudioStore(cfg),
			cfg.LLM.TTSVoiceName,
			cfg.LLM.TTSVoiceNameB,
		)
	}

	session := NewSessionService(SessionDeps{
		QuestionRepo:           repos.Question,
		SessionRepo:            repos.Session,
		SessionQuestionRepo:    repos.SessionQuestion,
		QuizActiveSessionRepo:  repos.QuizActiveSession,
		StudyActiveSessionRepo: repos.StudyActiveSession,
		MaterialRepo:           repos.Material,
		DB:                     db,
		UserRepo:               repos.User,
		Stores:                 stores,
		LLM:                    llm,
	})

	return &Services{
		Content:            NewContentService(repos.Content),
		User:               NewUserService(repos.User),
		Session:            session,
		MaterialPreference: NewMaterialPreferenceService(repos.MaterialPreference),
		Analyzer: NewAnalyzerService(
			repos.User,
			repos.SessionQuestion,
		),
		Tip: tip,
		LLMQuestion: NewLLMQuestionService(
			llm,
			tip,
			cfg.LLM.Model,
		),
		Audio: audioService,
	}
}
