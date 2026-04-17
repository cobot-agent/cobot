package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/cobot-agent/cobot/internal/bootstrap"
)

// handleSlashCommand processes slash commands and returns a tea.Cmd.
func (m *tuiModel) handleSlashCommand(text string) tea.Cmd {
	m.input.Reset()
	parts := strings.SplitN(text, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "/model":
		if arg == "" {
			m.messages = append(m.messages, chatMessage{
				role: "system", raw: fmt.Sprintf("Current model: %s", m.agent.Model()),
			})
			break
		}
		if err := m.agent.SetModel(arg); err != nil {
			m.messages = append(m.messages, chatMessage{
				role: "error", raw: fmt.Sprintf("Failed to set model: %v", err),
			})
		} else {
			m.messages = append(m.messages, chatMessage{
				role: "system", raw: fmt.Sprintf("Model switched to: %s", m.agent.Model()),
			})
		}

	case "/usage":
		u := m.agent.SessionUsage()
		info := fmt.Sprintf("Session usage — input: %d, output: %d, total: %d", u.PromptTokens, u.CompletionTokens, u.TotalTokens)
		if u.ReasoningTokens > 0 {
			info += fmt.Sprintf(", reasoning: %d", u.ReasoningTokens)
		}
		if u.CacheReadTokens > 0 || u.CacheWriteTokens > 0 {
			info += fmt.Sprintf(", cache_read: %d, cache_write: %d", u.CacheReadTokens, u.CacheWriteTokens)
		}
		m.messages = append(m.messages, chatMessage{
			role: "system",
			raw:  info,
		})

	case "/reset":
		m.agent.ResetUsage()
		m.messages = append(m.messages, chatMessage{role: "system", raw: "Usage counters reset."})

	case "/workspace":
		if m.wsMgr == nil {
			m.messages = append(m.messages, chatMessage{
				role: "error", raw: "Workspace manager not available.",
			})
			break
		}
		if arg == "" || arg == "list" {
			defs, err := m.wsMgr.List()
			if err != nil {
				m.messages = append(m.messages, chatMessage{
					role: "error", raw: fmt.Sprintf("Failed to list workspaces: %v", err),
				})
				break
			}
			if len(defs) == 0 {
				m.messages = append(m.messages, chatMessage{
					role: "system", raw: "No workspaces found.",
				})
				break
			}
			var lines []string
			for _, d := range defs {
				lines = append(lines, fmt.Sprintf("  %s (%s)", d.Name, d.Type))
			}
			m.messages = append(m.messages, chatMessage{
				role: "system", raw: "Workspaces:\n" + strings.Join(lines, "\n"),
			})
		} else {
			ws, err := m.wsMgr.Resolve(arg)
			if err != nil {
				m.messages = append(m.messages, chatMessage{
					role: "error", raw: fmt.Sprintf("Failed to resolve workspace: %v", err),
				})
				break
			}
			if err := bootstrap.ConfigureAgentForWorkspace(m.agent, ws, m.agent.Registry()); err != nil {
				m.messages = append(m.messages, chatMessage{
					role: "error", raw: fmt.Sprintf("Failed to switch workspace: %v", err),
				})
				break
			}
			m.workspace = ws.Definition.Name
			m.messages = append(m.messages, chatMessage{
				role: "system", raw: fmt.Sprintf("Switched to workspace: %s", ws.Definition.Name),
			})
		}

	case "/help":
		m.messages = append(m.messages, chatMessage{
			role: "system",
			raw:  "Commands: /model <spec>  /usage  /reset  /workspace [list|<name>]  /help  /quit",
		})

	default:
		m.messages = append(m.messages, chatMessage{
			role: "error", raw: fmt.Sprintf("Unknown command: %s  (try /help)", cmd),
		})
	}

	m.viewport.GotoBottom()
	m.refreshViewport()
	return nil
}
