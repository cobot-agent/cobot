package agent

import (
	"context"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// --- Context / prompt helpers ---

// buildMessages assembles the message list for the LLM, prepending the system
// prompt (cached LTM + fresh STM) as the first message.
func (a *Agent) buildMessages(ctx context.Context) []cobot.Message {
	msgs := a.session.Messages()
	system := a.getSystemPrompt(ctx)
	if system == "" {
		return msgs
	}

	// Append STM context on every turn (not cached like LTM).
	stmText := a.getSTMContext(ctx)
	if stmText != "" {
		system = system + "\n\n" + stmText
	}

	return append([]cobot.Message{{Role: cobot.RoleSystem, Content: system}}, msgs...)
}

// getSTMContext returns short-term memory text for the current session,
// refreshed every turn.
func (a *Agent) getSTMContext(ctx context.Context) string {
	if a.memoryStore == nil {
		return ""
	}
	stm, ok := a.memoryStore.(cobot.ShortTermMemory)
	if !ok {
		return ""
	}
	text, err := stm.WakeUpSTM(ctx, a.sessionID)
	if err != nil {
		return ""
	}
	return text
}

func (a *Agent) getSystemPrompt(ctx context.Context) string {
	a.sysPromptMu.RLock()
	cached := a.systemPrompt
	a.sysPromptMu.RUnlock()

	if cached != "" {
		return cached
	}

	if a.memoryRecall == nil {
		return cobot.DefaultSystemPrompt
	}

	// Double-check locking: acquire write lock and re-check to avoid
	// redundant WakeUp calls from concurrent cache misses.
	a.sysPromptMu.Lock()
	if a.systemPrompt != "" {
		a.sysPromptMu.Unlock()
		return a.systemPrompt
	}

	wakeUp, err := a.memoryRecall.WakeUp(ctx)
	if err != nil || wakeUp == "" {
		a.sysPromptMu.Unlock()
		return cobot.DefaultSystemPrompt
	}

	a.systemPrompt = wakeUp
	a.sysPromptMu.Unlock()

	return wakeUp
}
