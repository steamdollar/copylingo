package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// telegramClient owns every call to the Telegram Bot API: update polling,
// outbound messages/edits/voice, callback answers, and chat actions. Flows
// depend on it directly instead of reaching the API through *Bot.
type telegramClient struct {
	api BotAPI
}

func newTelegramClient(api BotAPI) *telegramClient {
	return &telegramClient{api: api}
}

// Updates starts long polling and returns the update channel.
func (c *telegramClient) Updates(pollConfig tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return c.api.GetUpdatesChan(pollConfig)
}

// StopUpdates stops long polling started by Updates.
func (c *telegramClient) StopUpdates() {
	c.api.StopReceivingUpdates()
}

// TODO: sendMessage, SendMessageWithKeyboard 굳이 따로 두는 이유가?
// SendMessage sends a text message to a chat.
func (c *telegramClient) SendMessage(
	chatID int64,
	text string,
) error {
	msg := tgbotapi.NewMessage(
		chatID,
		sanitizeTelegramHTML(text),
	)
	msg.ParseMode = "HTML"
	_, err := c.api.Send(msg)
	return err
}

// SendMessageWithKeyboard sends a message with an inline keyboard.
func (c *telegramClient) SendMessageWithKeyboard(
	chatID int64,
	text string,
	keyboard tgbotapi.InlineKeyboardMarkup,
) error {
	msg := tgbotapi.NewMessage(
		chatID,
		sanitizeTelegramHTML(text),
	)
	msg.ParseMode = "HTML"
	if len(keyboard.InlineKeyboard) > 0 {
		msg.ReplyMarkup = keyboard
	}
	_, err := c.api.Send(msg)
	return err
}

// SendMessageWithReplyMarkup sends a message with custom Telegram reply markup.
func (c *telegramClient) SendMessageWithReplyMarkup(
	chatID int64,
	text string,
	replyMarkup interface{},
) (int, error) {
	msg := tgbotapi.NewMessage(
		chatID,
		sanitizeTelegramHTML(text),
	)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = replyMarkup
	sent, err := c.api.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

// SendVoiceFileID sends a voice message by reusing a cached Telegram file_id.
func (c *telegramClient) SendVoiceFileID(
	chatID int64,
	fileID string,
) error {
	voice := tgbotapi.NewVoice(
		chatID,
		tgbotapi.FileID(fileID),
	)
	_, err := c.api.Send(voice)
	return err
}

// SendVoiceBytes uploads raw OGG/Opus bytes as a voice message and returns the
// Telegram file_id assigned to it, so callers can cache it for later re-sends.
func (c *telegramClient) SendVoiceBytes(
	chatID int64,
	data []byte,
) (string, error) {
	voice := tgbotapi.NewVoice(
		chatID,
		tgbotapi.FileBytes{Name: "listening.ogg", Bytes: data},
	)
	sent, err := c.api.Send(voice)
	if err != nil {
		return "", err
	}
	if sent.Voice != nil {
		return sent.Voice.FileID, nil
	}
	return "", nil
}

// EditMessageReplyMarkup updates the inline keyboard of an existing message.
func (c *telegramClient) EditMessageReplyMarkup(
	chatID int64,
	messageID int,
	markup tgbotapi.InlineKeyboardMarkup,
) error {
	edit := tgbotapi.NewEditMessageReplyMarkup(
		chatID,
		messageID,
		markup,
	)
	_, err := c.api.Send(edit)
	return err
}

// EditMessage edits an existing message.
func (c *telegramClient) EditMessage(
	chatID int64,
	messageID int,
	text string,
	keyboard *tgbotapi.InlineKeyboardMarkup,
) error {
	edit := tgbotapi.NewEditMessageText(
		chatID,
		messageID,
		sanitizeTelegramHTML(text),
	)
	edit.ParseMode = "HTML"
	if keyboard != nil && len(keyboard.InlineKeyboard) > 0 {
		edit.ReplyMarkup = keyboard
	}
	_, err := c.api.Send(edit)
	return err
}

// ClearInlineKeyboard removes inline buttons from an existing bot message.
func (c *telegramClient) ClearInlineKeyboard(
	chatID int64,
	messageID int,
) error {
	edit := tgbotapi.EditMessageReplyMarkupConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:    chatID,
			MessageID: messageID,
		},
	}
	_, err := c.api.Send(edit)
	return err
}

// AnswerCallback acknowledges a callback query with an optional toast text.
func (c *telegramClient) AnswerCallback(
	callbackID string,
	text string,
) error {
	_, err := c.api.Request(tgbotapi.NewCallback(
		callbackID,
		text,
	))
	return err
}

// AnswerCallbackAlert acknowledges a callback query with a modal alert.
func (c *telegramClient) AnswerCallbackAlert(
	callbackID string,
	text string,
) error {
	_, err := c.api.Request(tgbotapi.NewCallbackWithAlert(
		callbackID,
		text,
	))
	return err
}

// SendChatAction shows a transient status such as "typing" in the chat.
func (c *telegramClient) SendChatAction(
	chatID int64,
	action string,
) error {
	_, err := c.api.Request(tgbotapi.NewChatAction(
		chatID,
		action,
	))
	return err
}
