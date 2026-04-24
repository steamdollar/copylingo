package bot

type webAppInfo struct {
	URL string `json:"url"`
}

type webAppButton struct {
	Text         string      `json:"text"`
	WebApp       *webAppInfo `json:"web_app,omitempty"`
	CallbackData *string     `json:"callback_data,omitempty"`
}

type webAppKeyboardMarkup struct {
	InlineKeyboard [][]webAppButton `json:"inline_keyboard"`
}

func newWebAppButton(text, url string) webAppButton {
	return webAppButton{
		Text:   text,
		WebApp: &webAppInfo{URL: url},
	}
}

func newCallbackButton(text, data string) webAppButton {
	return webAppButton{
		Text:         text,
		CallbackData: &data,
	}
}
