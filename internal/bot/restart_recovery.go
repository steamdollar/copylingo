package bot

import (
	"context"
	"log/slog"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
)

// RefreshStaleMiniAppMessages is called once at server startup to check in-progress sessions.
// If the next unanswered question is a handwriting task and the Mini App URL has changed,
// it re-sends the question with a fresh URL.
func (b *Bot) RefreshStaleMiniAppMessages(ctx context.Context) {
	ctx = observability.WithAttrs(
		ctx,
		slog.String(
			"interaction_id",
			observability.NewInteractionID("job-restart_recovery"),
		),
		slog.String(
			"source",
			"telegram.restart_recovery",
		),
	)
	baseURL := b.cfg.Server.PublicBaseURL
	if baseURL == "" {
		slog.InfoContext(
			ctx,
			"Restart recovery skipped because public base URL is empty",
			"event",
			"telegram.restart_recovery.skipped",
		)
		return
	}
	currentFp := callback.MiniAppURLFingerprint(baseURL)

	sessions, err := b.services.SessionBuilder.GetAllInProgressSessions(ctx)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to list in-progress sessions for restart recovery",
			"event",
			"telegram.restart_recovery.session_list_failed",
			"error",
			err,
		)
		return
	}

	for _, s := range sessions {
		// Skip if fingerprint unchanged
		if b.recovery != nil {
			if last, _ := b.recovery.GetMiniAppFingerprint(
				ctx,
				s.ID,
			); last == currentFp {
				continue
			}
		}

		state, err := b.services.QuizActiveSession.Get(
			ctx,
			s.ID,
		)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"Active session state unavailable during restart recovery",
				"event",
				"telegram.restart_recovery.active_state_unavailable",
				"session_id",
				s.ID,
				"error",
				err,
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

		// Best-effort: edit the old message to strip its stale buttons.
		if b.messages != nil {
			if ref, err := b.messages.GetHandwritingMessage(
				ctx,
				s.ID,
				q.ID,
			); err == nil && ref != nil {
				_ = b.telegram.ClearInlineKeyboard(
					ref.ChatID,
					ref.MessageID,
				)
			}
		}

		// (b) re-send the question with fresh URL
		slog.InfoContext(
			ctx,
			"Refreshing stale handwriting link",
			"event",
			"telegram.restart_recovery.link_refreshing",
			"session_id",
			s.ID,
			"user_id",
			s.UserID,
		)
		b.telegram.SendMessage(
			s.UserID,
			botMessagesByLocale[botDefaultLocale].handwritingLinkUpdated,
		)
		b.flow.showQuestion(
			ctx,
			s.UserID,
			nil,
			s.ID,
			idx,
		)

		if b.recovery != nil {
			_ = b.recovery.SetMiniAppFingerprint(
				ctx,
				s.ID,
				currentFp,
			)
		}
	}
}
