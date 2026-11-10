package bot

import (
	"errors"
	"fmt"
	"log"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramClient struct {
	api BotAPI
}

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

func (client *TelegramClient) Updates(pollConfig tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return client.api.GetUpdatesChan(pollConfig)
}

func (client *TelegramClient) StopUpdates() {
	client.api.StopReceivingUpdates()
}

// send and request are the only Bot API paths. Failures are logged here because flows fire and forget
// (e.g. a 429 drops a keyboard).
// Methods take no context, so entries carry chat_id instead of interaction_id.
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

// Adds error_code and retry_after so rate limits are distinguishable from network errors.
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

// Returns the Telegram file_id so callers can cache it for re-sends.
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
