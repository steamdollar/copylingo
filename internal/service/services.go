package service

import (
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/config"
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
	srs := NewSRSService(repos.Question)
	grader := NewGraderService(repos, srs)
	analyzer := NewAnalyzerService(repos)
	sessionBuilder := NewSessionBuilderService(repos, srs)

	return &Services{
		SRS:            srs,
		SessionBuilder: sessionBuilder,
		Grader:         grader,
		Analyzer:       analyzer,
	}
}
