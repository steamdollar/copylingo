package bot

import (
	"context"
)

// PushSession sends a session notification to a user via Telegram.
// This is called by the scheduler to push daily sessions.
func (b *Bot) PushSession(ctx context.Context, chatID int64, sessionID int, sessionType string) error {
	return b.flow.PushSession(ctx, chatID, sessionID, sessionType)
}
