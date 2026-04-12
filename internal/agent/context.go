package agent

import (
	"context"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func (a *Agent) buildMessages(ctx context.Context) []cobot.Message {
	msgs := a.session.Messages()
	system := a.getSystemPrompt(ctx)
	if system == "" {
		return msgs
	}
	return append([]cobot.Message{{Role: cobot.RoleSystem, Content: system}}, msgs...)
}

func (a *Agent) getSystemPrompt(ctx context.Context) string {
	a.sysPromptMu.RLock()
	cached := a.systemPrompt
	a.sysPromptMu.RUnlock()

	if cached != "" {
		return cached
	}

	const defaultPrompt = "You are Cobot, a personal AI assistant."
	if a.memoryStore == nil {
		return defaultPrompt
	}

	wakeUp, err := a.memoryStore.WakeUp(ctx)
	if err != nil || wakeUp == "" {
		return defaultPrompt
	}

	a.sysPromptMu.Lock()
	a.systemPrompt = wakeUp
	a.sysPromptMu.Unlock()

	return wakeUp
}

func (a *Agent) invalidateSystemPrompt() {
	a.sysPromptMu.Lock()
	defer a.sysPromptMu.Unlock()
	a.systemPrompt = ""
}
