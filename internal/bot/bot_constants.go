package bot

import "github.com/lsj/copylingo/internal/callback"

const botDefaultLocale = "ko"

// Telegram commands and the LLM access list belong to the bot entry point.
type botCommand string

const (
	commandStart    botCommand = "start"
	commandMenu     botCommand = "menu"
	commandStats    botCommand = "stats"
	commandStreak   botCommand = "streak"
	commandStudy    botCommand = "study"
	commandLLM      botCommand = "llm"
	commandTest     botCommand = "test"
	commandHelp     botCommand = "help"
	commandExit     botCommand = "exit"
	commandSettings botCommand = "settings"
)

var llmAllowedTelegramUserIDs = [...]int64{2006481393}

// Callback identifiers stay separate from localized button text.
const (
	callbackRootMenu     = "menu"
	callbackRootSession  = "session"
	callbackRootStudy    = "study"
	callbackRootSettings = "settings"

	callbackPrefixMenu     = callbackRootMenu + ":"
	callbackPrefixSession  = callbackRootSession + ":"
	callbackPrefixStudy    = callbackRootStudy + ":"
	callbackPrefixSettings = callbackRootSettings + ":"
)

const (
	callbackActionStart   = "start"
	callbackActionFinish  = "finish"
	callbackActionPrev    = "prev"
	callbackActionAsk     = "ask"
	callbackActionExclude = "exclude"
	callbackActionCard    = "card"

	callbackActionWordOrder       = "wo"
	callbackActionWordOrderSelect = "a"
	callbackActionWordOrderUndo   = "u"
	callbackActionWordOrderReset  = "r"
	callbackActionWordOrderSubmit = "s"

	callbackActionSettingsTimezone = "set_tz"
	callbackActionSettingsSlot     = "slot"
	callbackActionSettingsSet      = "set"
	callbackActionMaterials        = "materials"
	callbackActionRestore          = "restore"
	callbackActionAllTimes         = "all"
	callbackActionOff              = "off"
)

const (
	callbackLLMCancel        = "llm:cancel"
	callbackMenuMain         = callbackPrefixMenu + "main"
	callbackMenuStudy        = callbackPrefixMenu + "study"
	callbackMenuReview       = callbackPrefixMenu + "review"
	callbackMenuStats        = callbackPrefixMenu + "stats"
	callbackMenuSettings     = callbackPrefixMenu + "settings"
	callbackSettingsView     = callbackPrefixSettings + "view"
	callbackSettingsTimezone = callbackPrefixSettings + "tz"
)

// These formats are consumed only by bot handlers and keyboards.
const (
	formatSessionStart      = callbackPrefixSession + "%d:" + callbackActionStart
	formatSessionFinish     = callbackPrefixSession + "%d:" + callbackActionFinish
	formatQuestionAnswer    = callback.QuestionPrefix + "%d:%d:%d"
	formatQuestionAskLLM    = callback.QuestionPrefix + "%d:" + callbackActionAsk + ":%d"
	formatQuestionPolicySet = callback.FormatQuestionPolicy + ":%s"

	formatWordOrderSelect = callback.QuestionPrefix + "%d:" + callbackActionWordOrder + ":%d:" + callbackActionWordOrderSelect + ":%d"
	formatWordOrderUndo   = callback.QuestionPrefix + "%d:" + callbackActionWordOrder + ":%d:" + callbackActionWordOrderUndo
	formatWordOrderReset  = callback.QuestionPrefix + "%d:" + callbackActionWordOrder + ":%d:" + callbackActionWordOrderReset
	formatWordOrderSubmit = callback.QuestionPrefix + "%d:" + callbackActionWordOrder + ":%d:" + callbackActionWordOrderSubmit

	formatStudyStart     = callbackPrefixStudy + "%d:" + callbackActionStart
	formatStudyNext      = callbackPrefixStudy + "%d:" + callback.QuestionActionNext + ":%d"
	formatStudyPrev      = callbackPrefixStudy + "%d:" + callbackActionPrev + ":%d"
	formatStudyFinish    = callbackPrefixStudy + "%d:" + callbackActionFinish + ":%d"
	formatStudyAskLLM    = callbackPrefixStudy + "%d:" + callbackActionAsk + ":%d"
	formatStudyPolicy    = callbackPrefixStudy + "%d:" + callback.QuestionActionPolicy + ":%d"
	formatStudyPolicySet = callbackPrefixStudy + "%d:" + callback.QuestionActionPolicy + ":%d:%s"
	formatStudyCard      = callbackPrefixStudy + "%d:" + callbackActionCard + ":%d"

	formatMaterialPreferences      = callbackPrefixSettings + callbackActionMaterials + ":%d"
	formatMaterialRestore          = callbackPrefixSettings + callbackActionRestore + ":%d:%d"
	formatSettingsTimezone         = callbackPrefixSettings + callbackActionSettingsTimezone + ":%s"
	callbackPrefixSettingsTimezone = callbackPrefixSettings + callbackActionSettingsTimezone + ":"
	formatSettingsSlot             = callbackPrefixSettings + callbackActionSettingsSlot + ":%s"
	callbackPrefixSettingsSlot     = callbackPrefixSettings + callbackActionSettingsSlot + ":"
	formatSettingsSlotAll          = callbackPrefixSettings + callbackActionSettingsSlot + ":%s:" + callbackActionAllTimes
	formatSettingsSet              = callbackPrefixSettings + callbackActionSettingsSet + ":%s:%s"
	callbackPrefixSettingsSet      = callbackPrefixSettings + callbackActionSettingsSet + ":"
)
