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
}

// NewServices creates all services with the given dependencies.
func NewServices(repos *repository.Repositories, cfg *config.Config, rdb redis.Cmdable) *Services {
	llm := external.NewLLMClient(cfg)

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
	}
}
