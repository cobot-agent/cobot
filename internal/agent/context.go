package agent

import (
	"context"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func (a *Agent) buildMessages(ctx context.Context) []cobot.Message {
	msgs := a.session.Messages()
	system := a.buildSystemPrompt(ctx)
	if system == "" {
		return msgs
	}
	return append([]cobot.Message{{Role: cobot.RoleSystem, Content: system}}, msgs...)
}

func (a *Agent) buildSystemPrompt(ctx context.Context) string {
	const defaultPrompt = "You are Cobot, a personal AI assistant."
	if a.memoryStore == nil {
		return defaultPrompt
	}
	wakeUp, err := a.memoryStore.WakeUp(ctx)
	if err != nil || wakeUp == "" {
		return defaultPrompt
	}
	return wakeUp
}
