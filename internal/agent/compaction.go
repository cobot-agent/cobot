package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func (a *Agent) checkAndCompress(ctx context.Context) {
	if a.compressor == nil {
		return
	}

	sm := a.sessionMgr
	action := a.compressor.Check(sm.usageTracker.Get(), sm.turnCount)
	if action == CompressNone {
		return
	}

	msgs, snapshotLen := sm.session.MessagesSnapshot()
	slog.Debug("compression triggered", "action", action, "turns", sm.turnCount, "total_tokens", sm.usageTracker.Get().TotalTokens, "messages", len(msgs))

	go a.runCompress(ctx, action, msgs, snapshotLen)
}

// promoteSTMBackground triggers an asynchronous STM→LTM promotion.
func (a *Agent) promoteSTMBackground(ctx context.Context) {
	sm := a.sessionMgr
	if sm.memoryStore == nil {
		return
	}
	stm, ok := sm.memoryStore.(cobot.ShortTermMemory)
	if !ok {
		return
	}
	go func() {
		if err := stm.SummarizeAndPromoteSTM(ctx, sm.sessionID); err != nil {
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
	sm := a.sessionMgr
	if stm, ok := sm.memoryStore.(cobot.ShortTermMemory); ok {
		if _, err := stm.StoreShortTermCompressed(ctx, sm.sessionID, summary); err != nil {
			slog.Debug("stm compressed store failed", "err", err)
		}
	}
}

func (a *Agent) replaceSessionMessages(summary string, kept []cobot.Message, snapshotLen int) {
	sm := a.sessionMgr
	sess := sm.session
	sess.mu.Lock()
	defer sess.mu.Unlock()

	var postSnapshot []cobot.Message
	if snapshotLen < len(sess.messages) {
		postSnapshot = make([]cobot.Message, len(sess.messages)-snapshotLen)
		copy(postSnapshot, sess.messages[snapshotLen:])
	}

	var newMsgs []cobot.Message
	if len(sess.messages) > 0 && sess.messages[0].Role == cobot.RoleSystem {
		newMsgs = append(newMsgs, sess.messages[0])
	}

	newMsgs = append(newMsgs, cobot.Message{
		Role:    cobot.RoleAssistant,
		Content: fmt.Sprintf("[Previous conversation summary]\n%s", summary),
	})
	newMsgs = append(newMsgs, kept...)
	newMsgs = append(newMsgs, postSnapshot...)
	originalCount := len(sess.messages)
	sess.messages = newMsgs

	newUsage := estimateMessagesUsage(newMsgs)
	sm.usageTracker.Set(newUsage)

	if sm.sessionStore != nil {
		sm.sessionStore.AppendCompact(sm.sessionID, CompactMarker{
			Summary:       summary,
			OriginalCount: originalCount,
		})
		sm.PersistUsage()
	}

	slog.Debug("session compressed", "original_msgs", originalCount, "new_msgs", len(newMsgs), "new_tokens", newUsage.TotalTokens)
}

// --- Memory extraction (post-compression) ---

func (a *Agent) extractMemories(ctx context.Context, summary string, originalMsgs []cobot.Message) {
	store := a.sessionMgr.memoryStore
	if store == nil || a.provider == nil {
		return
	}

	model := a.compressorModel()
	provider := a.provider

	go func() {
		if err := a.doExtractMemoriesWith(ctx, summary, originalMsgs, model, store, provider); err != nil {
			slog.Debug("memory extraction failed", "err", err)
		}
	}()
}

func (a *Agent) doExtractMemoriesWith(ctx context.Context, summary string, originalMsgs []cobot.Message, model string, store cobot.MemoryStore, provider cobot.Provider) error {
	var conversationBuf strings.Builder
	for i, m := range originalMsgs {
		if m.Role == cobot.RoleSystem {
			continue
		}
		if i >= 40 {
			conversationBuf.WriteString(fmt.Sprintf("\n... (%d more messages omitted)\n", len(originalMsgs)-40))
			break
		}
		conversationBuf.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, cobot.Truncate(m.Content, 300)))
		for _, tc := range m.ToolCalls {
			conversationBuf.WriteString(fmt.Sprintf("  tool_call: %s\n", tc.Name))
		}
		if m.ToolResult != nil && m.ToolResult.Output != "" {
			conversationBuf.WriteString(fmt.Sprintf("  tool_result: %s\n", cobot.Truncate(m.ToolResult.Output, 200)))
		}
	}

	userContent := fmt.Sprintf(
		"<summary>\n%s\n</summary>\n\n<conversation>\n%s\n</conversation>",
		summary,
		conversationBuf.String(),
	)

	req := &cobot.ProviderRequest{
		Model: model,
		Messages: []cobot.Message{
			{Role: cobot.RoleSystem, Content: memoryExtractionPrompt},
			{Role: cobot.RoleUser, Content: userContent},
		},
	}

	resp, err := provider.Complete(ctx, req)
	if err != nil {
		return fmt.Errorf("memory extraction LLM call: %w", err)
	}

	items := parseExtractionResponse(resp.Content)
	if len(items) == 0 {
		slog.Debug("memory extraction: no items extracted")
		return nil
	}

	stored := 0
	rooms := make(map[string]struct{})
	for _, item := range items {
		_, err := store.StoreByName(ctx, item.content, "sessions", item.room, item.hallType)
		if err != nil {
			slog.Debug("memory extraction: store failed", "room", item.room, "err", err)
			continue
		}
		rooms[item.room] = struct{}{}
		stored++
	}

	for room := range rooms {
		if err := store.ConsolidateByName(ctx, "sessions", room); err != nil {
			slog.Debug("memory consolidation failed", "room", room, "err", err)
		}
	}

	slog.Debug("memory extraction complete", "extracted", len(items), "stored", stored)
	return nil
}

func (a *Agent) compressorModel() string {
	if a.compressor != nil && a.compressor.summaryModel != "" {
		return a.compressor.summaryModel
	}
	return a.config.Model
}

type memoryItem struct {
	content  string
	room     string
	hallType string
}

// parseExtractionResponse parses the structured LLM response into memory items.
// Expected format: [FACT], [DECISION], [PATTERN], [PREFERENCE] prefixed lines.
func parseExtractionResponse(response string) []memoryItem {
	lines := strings.Split(strings.TrimSpace(response), "\n")
	var items []memoryItem

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var room, hallType string
		switch {
		case strings.HasPrefix(line, "[FACT]"):
			line = strings.TrimPrefix(line, "[FACT]")
			room = "facts"
			hallType = cobot.TagFacts
		case strings.HasPrefix(line, "[DECISION]"):
			line = strings.TrimPrefix(line, "[DECISION]")
			room = "decisions"
			hallType = cobot.TagFacts
		case strings.HasPrefix(line, "[PATTERN]"):
			line = strings.TrimPrefix(line, "[PATTERN]")
			room = "patterns"
			hallType = cobot.TagCode
		case strings.HasPrefix(line, "[PREFERENCE]"):
			line = strings.TrimPrefix(line, "[PREFERENCE]")
			room = "preferences"
			hallType = cobot.TagFacts
		default:
			continue
		}

		content := strings.TrimSpace(line)
		if content == "" {
			continue
		}

		items = append(items, memoryItem{
			content:  content,
			room:     room,
			hallType: hallType,
		})
	}

	return items
}

const memoryExtractionPrompt = `You are a memory extraction engine. Given a conversation summary and the original conversation, extract the most important items worth remembering for future sessions.

Extract ONLY items that are:
- Durable: still relevant in future conversations (not ephemeral status updates)
- Specific: contain concrete names, paths, numbers, or decisions (not vague observations)
- Actionable: inform future behavior or decisions

Categorize each item with exactly one tag:
- [FACT] — A concrete fact about the project, codebase, user, or environment
- [DECISION] — A decision made and its rationale
- [PATTERN] — A code pattern, convention, or architectural approach established
- [PREFERENCE] — A user preference about workflow, style, or tooling

Output one item per line, prefixed with its tag. No numbering, no bullets, no commentary.
If nothing is worth extracting, output a single line: NONE

Examples:
[FACT] The project uses SQLite with WAL mode for the MemPalace memory backend
[DECISION] Session compression threshold set to 70% of context window because lower values caused too-frequent compression
[PATTERN] Error handling in providers follows: wrap with fmt.Errorf("method: %w", err), never suppress
[PREFERENCE] User prefers Chinese for discussion but English for code comments`
