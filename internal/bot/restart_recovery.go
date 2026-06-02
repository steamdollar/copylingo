package bot

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
)

// RefreshStaleMiniAppMessages is called once at server startup to check in-progress sessions.
// If the next unanswered question is a handwriting task and the Mini App URL has changed,
// it re-sends the question with a fresh URL.
func (b *Bot) RefreshStaleMiniAppMessages(ctx context.Context) {
	ctx = observability.WithAttrs(ctx,
		slog.String("interaction_id", observability.NewInteractionID("job-restart_recovery")),
		slog.String("source", "telegram.restart_recovery"),
	)
	baseURL := b.cfg.Server.PublicBaseURL
	if baseURL == "" {
		slog.InfoContext(ctx, "Restart recovery skipped because public base URL is empty",
			"event", "telegram.restart_recovery.skipped",
		)
		return
	}
	currentFp := callback.MiniAppURLFingerprint(baseURL)

	sessions, err := b.services.SessionBuilder.GetAllInProgressSessions(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to list in-progress sessions for restart recovery",
			"event", "telegram.restart_recovery.session_list_failed",
			"error", err,
		)
		return
	}

	for _, s := range sessions {
		// Skip if fingerprint unchanged
		key := fmt.Sprintf("copylingo:miniapp:last_fingerprint:%d", s.ID)
		if last, _ := b.rdb.Get(ctx, key).Result(); last == currentFp {
			continue
		}

		state, err := b.services.ActiveSession.Get(ctx, s.ID)
		if err != nil {
			slog.ErrorContext(ctx, "Active session state unavailable during restart recovery",
				"event", "telegram.restart_recovery.active_state_unavailable",
				"session_id", s.ID,
				"error", err,
			)
			continue
		}

		idx := state.NextUnansweredIndex()
		if idx >= len(state.Items) {
			continue
		}

		q := state.Items[idx].Question
		if q.Type != model.QuestionKanaHandwriting {
			continue
		}

		// (a) best-effort: edit old message to strip buttons via HandwritingMessageRedisKey
		oldKey := config.HandwritingMessageRedisKey.Format(s.ID, q.ID)
		if val, err := b.rdb.Get(ctx, oldKey).Result(); err == nil {
			if chatID, msgID, perr := ParseHandwritingMessageRef(val); perr == nil {
				_ = b.ClearInlineKeyboard(chatID, msgID)
			}
		}

		// (b) re-send the question with fresh URL
		slog.InfoContext(ctx, "Refreshing stale handwriting link",
			"event", "telegram.restart_recovery.link_refreshing",
			"session_id", s.ID,
			"user_id", s.UserID,
		)
		b.SendMessage(s.UserID, "🔄 손글씨 링크가 갱신되었습니다. 아래 버튼으로 다시 진행해 주세요.")
		b.flow.showQuestion(ctx, s.UserID, nil, s.ID, idx)

		_ = b.rdb.Set(ctx, key, currentFp, 24*time.Hour).Err()
	}
}
