package bot

import (
	"errors"
	"fmt"
	"log"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramClient owns every call to the Telegram Bot API: update polling,
// outbound messages/edits/voice, callback answers, and chat actions. Flows
// depend on it directly instead of reaching the API through *Bot.
type TelegramClient struct {
	api BotAPI
}

// NewTelegramClient authorizes the bot token against the Telegram Bot API.
// cmd/server creates one client and shares it with every flow and the router.
func NewTelegramClient(
	token string,
	debug bool,
) (*TelegramClient, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create Telegram bot: %w",
			err,
		)
	}

	api.Debug = debug
	log.Printf(
		"Telegram bot authorized as @%s",
		api.Self.UserName,
	)
	return newTelegramClient(api), nil
}

func newTelegramClient(api BotAPI) *TelegramClient {
	return &TelegramClient{api: api}
}

// Updates starts long polling and returns the update channel.
func (client *TelegramClient) Updates(pollConfig tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return client.api.GetUpdatesChan(pollConfig)
}

// StopUpdates stops long polling started by Updates.
func (client *TelegramClient) StopUpdates() {
	client.api.StopReceivingUpdates()
}

// send and request are the only paths to the Bot API. A failed call is logged
// here because most flows fire and forget outbound messages; without this a
// rejected send (e.g. 429 dropping a "next question" keyboard) leaves no trace.
// The methods take no context, so entries carry the target instead of
// interaction_id; correlate them with the update log by time and chat_id.
func (client *TelegramClient) send(
	method string,
	target slog.Attr,
	chattable tgbotapi.Chattable,
) (tgbotapi.Message, error) {
	sent, err := client.api.Send(chattable)
	if err != nil {
		logTelegramAPIFailure(
			method,
			target,
			err,
		)
	}
	return sent, err
}

func (client *TelegramClient) request(
	method string,
	target slog.Attr,
	chattable tgbotapi.Chattable,
) error {
	_, err := client.api.Request(chattable)
	if err != nil {
		logTelegramAPIFailure(
			method,
			target,
			err,
		)
	}
	return err
}

// logTelegramAPIFailure adds Telegram's error_code and retry_after when the
// API rejected the call, so rate limits are distinguishable from network errors.
func logTelegramAPIFailure(
	method string,
	target slog.Attr,
	err error,
) {
	attributes := []any{
		"event",
		"telegram.api.failed",
		"method",
		method,
		target,
		"error",
		err,
	}
	var apiErr *tgbotapi.Error
	if errors.As(
		err,
		&apiErr,
	) {
		attributes = append(
			attributes,
			"error_code",
			apiErr.Code,
		)
		if apiErr.RetryAfter > 0 {
			attributes = append(
				attributes,
				"retry_after_sec",
				apiErr.RetryAfter,
			)
		}
	}
	slog.Warn(
		"Telegram API call failed",
		attributes...,
	)
}

func chatTarget(chatID int64) slog.Attr {
	return slog.Int64(
		"chat_id",
		chatID,
	)
}

// TODO: sendMessage, SendMessageWithKeyboard 굳이 따로 두는 이유가?
// SendMessage sends a text message to a chat.
func (client *TelegramClient) SendMessage(
	chatID int64,
	text string,
) error {
	message := tgbotapi.NewMessage(
		chatID,
		sanitizeTelegramHTML(text),
	)
	message.ParseMode = "HTML"
	_, err := client.send(
		"sendMessage",
		chatTarget(chatID),
		message,
	)
	return err
}

// SendMessageWithKeyboard sends a message with an inline keyboard.
func (client *TelegramClient) SendMessageWithKeyboard(
	chatID int64,
	text string,
	keyboard tgbotapi.InlineKeyboardMarkup,
) error {
	message := tgbotapi.NewMessage(
		chatID,
		sanitizeTelegramHTML(text),
	)
	message.ParseMode = "HTML"
	if len(keyboard.InlineKeyboard) > 0 {
		message.ReplyMarkup = keyboard
	}
	_, err := client.send(
		"sendMessage",
		chatTarget(chatID),
		message,
	)
	return err
}

// SendMessageWithReplyMarkup sends a message with custom Telegram reply markup.
func (client *TelegramClient) SendMessageWithReplyMarkup(
	chatID int64,
	text string,
	replyMarkup interface{},
) (int, error) {
	message := tgbotapi.NewMessage(
		chatID,
		sanitizeTelegramHTML(text),
	)
	message.ParseMode = "HTML"
	message.ReplyMarkup = replyMarkup
	sent, err := client.send(
		"sendMessage",
		chatTarget(chatID),
		message,
	)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

// SendVoiceFileID sends a voice message by reusing a cached Telegram file_id.
func (client *TelegramClient) SendVoiceFileID(
	chatID int64,
	fileID string,
) error {
	voice := tgbotapi.NewVoice(
		chatID,
		tgbotapi.FileID(fileID),
	)
	_, err := client.send(
		"sendVoice",
		chatTarget(chatID),
		voice,
	)
	return err
}

// SendVoiceBytes uploads raw OGG/Opus bytes as a voice message and returns the
// Telegram file_id assigned to it, so callers can cache it for later re-sends.
func (client *TelegramClient) SendVoiceBytes(
	chatID int64,
	data []byte,
) (string, error) {
	voice := tgbotapi.NewVoice(
		chatID,
		tgbotapi.FileBytes{Name: "listening.ogg", Bytes: data},
	)
	sent, err := client.send(
		"sendVoice",
		chatTarget(chatID),
		voice,
	)
	if err != nil {
		return "", err
	}
	if sent.Voice != nil {
		return sent.Voice.FileID, nil
	}
	return "", nil
}

// EditMessageReplyMarkup updates the inline keyboard of an existing message.
func (client *TelegramClient) EditMessageReplyMarkup(
	chatID int64,
	messageID int,
	markup tgbotapi.InlineKeyboardMarkup,
) error {
	edit := tgbotapi.NewEditMessageReplyMarkup(
		chatID,
		messageID,
		markup,
	)
	_, err := client.send(
		"editMessageReplyMarkup",
		chatTarget(chatID),
		edit,
	)
	return err
}

// DeleteMessage removes a bot message, e.g. a dismissed settings menu.
func (client *TelegramClient) DeleteMessage(
	chatID int64,
	messageID int,
) error {
	return client.request(
		"deleteMessage",
		chatTarget(chatID),
		tgbotapi.NewDeleteMessage(
			chatID,
			messageID,
		),
	)
}

// EditMessage edits an existing message.
func (client *TelegramClient) EditMessage(
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
	_, err := client.send(
		"editMessageText",
		chatTarget(chatID),
		edit,
	)
	return err
}

// ClearInlineKeyboard removes inline buttons from an existing bot message.
func (client *TelegramClient) ClearInlineKeyboard(
	chatID int64,
	messageID int,
) error {
	edit := tgbotapi.EditMessageReplyMarkupConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:    chatID,
			MessageID: messageID,
		},
	}
	_, err := client.send(
		"editMessageReplyMarkup",
		chatTarget(chatID),
		edit,
	)
	return err
}

// AnswerCallback acknowledges a callback query with an optional toast text.
func (client *TelegramClient) AnswerCallback(
	callbackID string,
	text string,
) error {
	return client.request(
		"answerCallbackQuery",
		slog.String(
			"callback_query_id",
			callbackID,
		),
		tgbotapi.NewCallback(
			callbackID,
			text,
		),
	)
}

// AnswerCallbackAlert acknowledges a callback query with a modal alert.
func (client *TelegramClient) AnswerCallbackAlert(
	callbackID string,
	text string,
) error {
	return client.request(
		"answerCallbackQuery",
		slog.String(
			"callback_query_id",
			callbackID,
		),
		tgbotapi.NewCallbackWithAlert(
			callbackID,
			text,
		),
	)
}

// SendChatAction shows a transient status such as "typing" in the chat.
func (client *TelegramClient) SendChatAction(
	chatID int64,
	action string,
) error {
	return client.request(
		"sendChatAction",
		chatTarget(chatID),
		tgbotapi.NewChatAction(
			chatID,
			action,
		),
	)
}
