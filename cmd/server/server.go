package main

import (
	"log"

	"github.com/lsj/copylingo/internal/bot"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/pipeline"
	"github.com/lsj/copylingo/internal/redisstore"
	"github.com/lsj/copylingo/internal/service"
)

// botComponents is the Telegram side initApp assembles. It stays inside
// cmd/server: each consumer receives only the flow it calls (ADR-059 §8.3).
type botComponents struct {
	router      *bot.Bot
	sessionFlow *bot.SessionFlow
	studyFlow   *bot.StudyFlow
}

// initBot creates one Telegram client, each feature flow over the services
// and Redis interaction state it uses, and the router over the flows.
func initBot(
	cfg *config.Config,
	svc services,
	interactions *redisstore.Interactions,
) (botComponents, error) {
	telegram, err := bot.NewTelegramClient(
		cfg.Telegram.Token,
		cfg.Telegram.Debug,
	)
	if err != nil {
		return botComponents{}, err
	}

	studyFlow := bot.NewStudyFlow(bot.StudyFlowDeps{
		Telegram:           telegram,
		Session:            svc.session,
		MaterialPreference: svc.materialPreference,
		Input:              interactions,
	})
	sessionDeps := bot.SessionFlowDeps{
		Telegram:           telegram,
		Session:            svc.session,
		User:               svc.user,
		MaterialPreference: svc.materialPreference,
		Input:              interactions,
		Drafts:             interactions,
		Messages:           interactions,
		Recovery:           interactions,
		Timing:             interactions,
		Study:              studyFlow,
		PublicBaseURL:      cfg.Server.PublicBaseURL,
	}
	// Audio is nil without a TTS key; assigning a nil *AudioService would make
	// a non-nil interface and bypass SessionFlow's "audio unavailable" path.
	if svc.audio != nil {
		sessionDeps.Audio = svc.audio
	}
	sessionFlow := bot.NewSessionFlow(sessionDeps)

	router := bot.NewBot(bot.BotDeps{
		Telegram:    telegram,
		User:        svc.user,
		Session:     svc.session,
		Analyzer:    svc.analyzer,
		Input:       interactions,
		SessionFlow: sessionFlow,
		StudyFlow:   studyFlow,
		SettingsFlow: bot.NewSettingsFlow(bot.SettingsFlowDeps{
			Telegram:           telegram,
			User:               svc.user,
			MaterialPreference: svc.materialPreference,
		}),
		LLMQuestionFlow: bot.NewLLMQuestionFlow(bot.LLMQuestionFlowDeps{
			Telegram:    telegram,
			User:        svc.user,
			LLMQuestion: svc.llmQuestion,
			Session:     svc.session,
			Input:       interactions,
		}),
	})
	return botComponents{
		router:      router,
		sessionFlow: sessionFlow,
		studyFlow:   studyFlow,
	}, nil
}

// initPipeline is kept for re-enabling content collection (ADR-057): startup
// does not build it, and a caller creates ContentService on that path.
func initPipeline(content *service.ContentService) *pipeline.Orchestrator {
	// NHK News Easy pipeline
	nhkClient := external.NewNHKClient()
	nhkFetcher := pipeline.NewNHKFetcher(nhkClient)
	processor := pipeline.NewPassThroughProcessor()
	saver := content

	orchestrator := pipeline.NewOrchestrator()
	orchestrator.Register(
		nhkFetcher,
		processor,
		saver,
	)

	log.Println("Content collection pipeline initialized")
	return orchestrator
}
