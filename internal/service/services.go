package service

import (
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/repository"
)

// Services holds all service instances.
type Services struct {
	User           *UserService
	SRS            *SRSService
	SessionBuilder *SessionBuilderService
	Grader         *GraderService
	Analyzer       *AnalyzerService
}

// NewServices creates all services with the given dependencies.
func NewServices(repos *repository.Repositories, cfg *config.Config) *Services {
	llm := external.NewLLMClient(cfg)

	userService := NewUserService(repos.User)
	srsService := NewSRSService(repos.Question)
	graderService := NewGraderService(repos.User,
		repos.Question, repos.Session, repos.SessionQuestion, srsService, llm)
	analyzerService := NewAnalyzerService(repos.User, repos.SessionQuestion)
	sessionBuilderService := NewSessionBuilderService(repos.Question,
		repos.Session, repos.SessionQuestion, srsService)

	return &Services{
		User:           userService,
		SRS:            srsService,
		SessionBuilder: sessionBuilderService,
		Grader:         graderService,
		Analyzer:       analyzerService,
	}
}
