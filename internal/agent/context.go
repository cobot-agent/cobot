package agent

import (
	cobot "github.com/cobot-agent/cobot/pkg"
)

func (a *Agent) buildMessages(userMessage string) []cobot.Message {
	msgs := a.session.Messages()
	msgs = append(msgs, cobot.Message{
		Role:    cobot.RoleUser,
		Content: userMessage,
	})
	return msgs
}
