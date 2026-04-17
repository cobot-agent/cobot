package agent

import (
	"context"
	"fmt"
	"log/slog"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func (a *Agent) checkAndCompress(ctx context.Context) {
	if a.compressor == nil {
		return
	}

	action := a.compressor.Check(a.usageTracker.Get(), a.turnCount)
	if action == CompressNone {
		return
	}

	msgs, snapshotLen := a.session.MessagesSnapshot()
	slog.Debug("compression triggered", "action", action, "turns", a.turnCount, "total_tokens", a.usageTracker.Get().TotalTokens, "messages", len(msgs))

	go a.runCompress(ctx, action, msgs, snapshotLen)
}

// promoteSTMBackground triggers an asynchronous STM→LTM promotion.
func (a *Agent) promoteSTMBackground(ctx context.Context) {
	if a.memoryStore == nil {
		return
	}
	stm, ok := a.memoryStore.(cobot.ShortTermMemory)
	if !ok {
		return
	}
	go func() {
		if err := stm.SummarizeAndPromoteSTM(ctx, a.sessionID); err != nil {
			slog.Debug("periodic STM promotion failed", "err", err)
		}
	}()
}

func (a *Agent) runCompress(ctx context.Context, action CompressAction, msgs []cobot.Message, snapshotLen int) {
	if !a.compressMu.TryLock() {
		slog.Debug("compression already in progress, skipping")
		return
	}
	defer a.compressMu.Unlock()

	var summary string
	var kept []cobot.Message

	switch action {
	case CompressSummarize:
		var err error
		summary, kept, err = a.compressor.Summarize(ctx, msgs)
		if err != nil {
			slog.Debug("summarize failed", "err", err)
			return
		}
	case CompressFull:
		var err error
		summary, err = a.compressor.Compress(ctx, msgs)
		if err != nil {
			slog.Debug("compress failed", "err", err)
			return
		}
	}

	optimized, err := a.compressor.OptimizeSummary(ctx, summary, msgs)
	if err == nil && optimized != "" {
		summary = optimized
	}
	a.replaceSessionMessages(summary, kept, snapshotLen)
	a.extractMemories(ctx, summary, msgs)
	// Store compression summary in STM compressed room.
	if stm, ok := a.memoryStore.(cobot.ShortTermMemory); ok {
		if _, err := stm.StoreShortTermCompressed(ctx, a.sessionID, summary); err != nil {
			slog.Debug("stm compressed store failed", "err", err)
		}
	}
}

func (a *Agent) replaceSessionMessages(summary string, kept []cobot.Message, snapshotLen int) {
	a.session.mu.Lock()
	defer a.session.mu.Unlock()

	// Collect messages appended after the snapshot was taken so they aren't lost.
	var postSnapshot []cobot.Message
	if snapshotLen < len(a.session.messages) {
		postSnapshot = make([]cobot.Message, len(a.session.messages)-snapshotLen)
		copy(postSnapshot, a.session.messages[snapshotLen:])
	}

	var newMsgs []cobot.Message
	if len(a.session.messages) > 0 && a.session.messages[0].Role == cobot.RoleSystem {
		newMsgs = append(newMsgs, a.session.messages[0])
	}

	newMsgs = append(newMsgs, cobot.Message{
		Role:    cobot.RoleAssistant,
		Content: fmt.Sprintf("[Previous conversation summary]\n%s", summary),
	})
	newMsgs = append(newMsgs, kept...)
	newMsgs = append(newMsgs, postSnapshot...)
	originalCount := len(a.session.messages)
	a.session.messages = newMsgs

	newUsage := estimateMessagesUsage(newMsgs)
	a.usageTracker.Set(newUsage)

	if a.sessionStore != nil {
		a.sessionStore.AppendCompact(a.sessionID, CompactMarker{
			Summary:       summary,
			OriginalCount: originalCount,
		})
		a.PersistUsage()
	}

	slog.Debug("session compressed", "original_msgs", originalCount, "new_msgs", len(newMsgs), "new_tokens", newUsage.TotalTokens)
}
