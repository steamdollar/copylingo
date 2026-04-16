package service

import (
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/repository"
)

// Services holds all service instances.
type Services struct {
	SRS            *SRSService
	SessionBuilder *SessionBuilderService
	Grader         *GraderService
	Analyzer       *AnalyzerService
}

// NewServices creates all services with the given dependencies.
func NewServices(repos *repository.Repositories, rdb *redis.Client, cfg *config.Config) *Services {
	llm := external.NewLLMClient(cfg)

	srsService := NewSRSService(repos.Question)
	graderService := NewGraderService(repos, srsService, llm)
	analyzerService := NewAnalyzerService(repos)
	sessionBuilderService := NewSessionBuilderService(repos, srsService)

	return &Services{
		SRS:            srsService,
		SessionBuilder: sessionBuilderService,
		Grader:         graderService,
		Analyzer:       analyzerService,
	}
}
