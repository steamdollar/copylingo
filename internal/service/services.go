package service

import (
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/repository"
)

// Services holds all service instances.
type Services struct {
	User               *UserService
	SRS                *SRSService
	SessionBuilder     *SessionBuilderService
	StudySession       *StudySessionService
	StudyActiveSession *StudyActiveSessionService
	ActiveSession      *ActiveSessionService
	Grader             *GraderService
	Handwriting        *HandwritingService
	Analyzer           *AnalyzerService
	Tip                *TipService
	TipGenerator       *TipGenerator
	LLM                *LLMService
}

// NewServices creates all services with the given dependencies.
func NewServices(repos *repository.Repositories, cfg *config.Config, rdb redis.Cmdable) *Services {
	// Share one LLM client between grading (LLMService) and tip generation.
	// GenerateTips lives on the concrete *DefaultLLMClient (not the LLMClient
	// interface), so assert to reach it.
	llmClient := external.NewLLMClient(cfg)
	llm := NewLLMService(llmClient)

	// Build the tip generator only when the concrete client is available; a typed
	// nil would defeat TopUpBucket's nil guard, so leave the field nil otherwise
	// (the scheduler already tolerates a nil TipGenerator).
	tipGenerator := newTipGeneratorFromClient(repos.Tip, llmClient, cfg.LLM.Model)

	srsService := NewSRSService(repos.Question)
	activeSessionService := NewActiveSessionService(repos.ActiveSession, rdb, srsService)
	graderService := NewGraderService(repos.User, activeSessionService, llm)

	return &Services{
		User: NewUserService(repos.User),
		SRS:  srsService,
		SessionBuilder: NewSessionBuilderService(repos.Question,
			repos.Session, repos.SessionQuestion, srsService),
		StudySession:       NewStudySessionService(repos.Material, repos.Session, repos.SessionMaterial),
		StudyActiveSession: NewStudyActiveSessionService(repos.StudyActiveSession, repos.Session, rdb),
		ActiveSession:      activeSessionService,
		Grader:             graderService,
		Handwriting:        NewHandwritingService(activeSessionService, graderService, nil),
		Analyzer:           NewAnalyzerService(repos.User, repos.SessionQuestion),
		Tip:                NewTipService(repos.Tip),
		TipGenerator:       tipGenerator,
		LLM:                llm,
	}
}
