package main

import (
	"github.com/lsj/copylingo/internal/bot"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/redisstore"
)

// botComponents is the Telegram side initializeApp assembles; each consumer receives only the flow it calls.
type botComponents struct {
	router      *bot.Bot
	sessionFlow *bot.SessionFlow
	studyFlow   *bot.StudyFlow
}

func initializeBot(
	appConfig *config.Config,
	appServices services,
	interactions *redisstore.Interactions,
) (botComponents, error) {
	telegram, err := bot.NewTelegramClient(
		appConfig.Telegram.Token,
		appConfig.Telegram.Debug,
	)
	if err != nil {
		return botComponents{}, err
	}

	studyFlow := bot.NewStudyFlow(bot.StudyFlowDependencies{
		Telegram:           telegram,
		Session:            appServices.session,
		MaterialPreference: appServices.materialPreference,
		Input:              interactions,
	})
	sessionFlow := bot.NewSessionFlow(bot.SessionFlowDependencies{
		Telegram:           telegram,
		Session:            appServices.session,
		User:               appServices.user,
		MaterialPreference: appServices.materialPreference,
		Input:              interactions,
		Drafts:             interactions,
		Messages:           interactions,
		Recovery:           interactions,
		Study:              studyFlow,
		Audio:              appServices.audio,
		PublicBaseURL:      appConfig.Server.PublicBaseURL,
	})

	router := bot.NewBot(bot.BotDependencies{
		Telegram:    telegram,
		User:        appServices.user,
		Session:     appServices.session,
		Analyzer:    appServices.analyzer,
		Input:       interactions,
		SessionFlow: sessionFlow,
		StudyFlow:   studyFlow,
		SettingsFlow: bot.NewSettingsFlow(bot.SettingsFlowDependencies{
			Telegram:           telegram,
			User:               appServices.user,
			MaterialPreference: appServices.materialPreference,
		}),
		LLMQuestionFlow: bot.NewLLMQuestionFlow(bot.LLMQuestionFlowDependencies{
			Telegram:    telegram,
			User:        appServices.user,
			LLMQuestion: appServices.llmQuestion,
			Session:     appServices.session,
			Input:       interactions,
		}),
	})
	return botComponents{
		router:      router,
		sessionFlow: sessionFlow,
		studyFlow:   studyFlow,
	}, nil
}
