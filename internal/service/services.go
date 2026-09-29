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
	StudySession       *StudySessionService
	StudyActiveSession *StudyActiveSessionService
	MaterialPreference *MaterialPreferenceService
	Analyzer           *AnalyzerService
	Tip                *TipService
	TipGenerator       *TipGenerator
	Audio              *AudioService
	LLM                *LLMService
}

// NewServices creates all services with the given dependencies.
func NewServices(
	repos *repository.Repositories,
	db *sqlx.DB,
	cfg *config.Config,
	stores SessionStores,
) *Services {
	// Share one LLM client between grading (LLMService) and tip generation.
	// GenerateTips lives on the concrete *DefaultLLMClient (not the LLMClient
	// interface), so assert to reach it.
	llmClient := external.NewLLMClient(cfg)
	llm := NewLLMService(llmClient)

	// Build the tip generator only when the concrete client is available; a typed
	// nil would defeat TopUpBucket's nil guard, so leave the field nil otherwise
	// (the scheduler already tolerates a nil TipGenerator).
	tipGenerator := newTipGeneratorFromClient(
		repos.Tip,
		llmClient,
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
		QuestionRepo:          repos.Question,
		SessionRepo:           repos.Session,
		SessionQuestionRepo:   repos.SessionQuestion,
		QuizActiveSessionRepo: repos.QuizActiveSession,
		UserRepo:              repos.User,
		Stores:                stores,
		LLM:                   llm,
	})

	return &Services{
		Content: NewContentService(repos.Content),
		User:    NewUserService(repos.User),
		Session: session,
		StudySession: NewStudySessionService(
			repos.Material,
			repos.Session,
			db,
		),
		StudyActiveSession: NewStudyActiveSessionService(
			repos.StudyActiveSession,
			repos.Session,
			stores.Study,
		),
		MaterialPreference: NewMaterialPreferenceService(repos.MaterialPreference),
		Analyzer: NewAnalyzerService(
			repos.User,
			repos.SessionQuestion,
		),
		Tip:          NewTipService(repos.Tip),
		TipGenerator: tipGenerator,
		Audio:        audioService,
		LLM:          llm,
	}
}
