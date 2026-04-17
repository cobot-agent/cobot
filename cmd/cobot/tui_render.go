package main

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/muesli/reflow/wrap"
)

// initStyles creates lipgloss styles.
func initStyles() (user, errSt, tool, status, queued, hub lipgloss.Style) {
	user = lipgloss.NewStyle().Foreground(lipgloss.Color("#87CEEB")).Bold(true)
	errSt = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	tool = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Faint(true)
	status = lipgloss.NewStyle().Faint(true)
	queued = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Faint(true)
	hub = lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Faint(true)
	return
}

func newGlamourRenderer(width int) *glamour.TermRenderer {
	w := width - 2 // leave some margin
	if w < 40 {
		w = 40
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(w),
	)
	if err != nil {
		// Fallback: no markdown rendering if glamour fails
		return nil
	}
	return r
}

func transparentTextareaStyles() textarea.Styles {
	var s textarea.Styles
	s.Focused = textarea.StyleState{
		Base:             lipgloss.NewStyle(),
		CursorLine:       lipgloss.NewStyle(),
		CursorLineNumber: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		EndOfBuffer:      lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
		LineNumber:       lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Placeholder:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Prompt:           lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
		Text:             lipgloss.NewStyle(),
	}
	s.Blurred = textarea.StyleState{
		Base:             lipgloss.NewStyle(),
		CursorLine:       lipgloss.NewStyle(),
		CursorLineNumber: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		EndOfBuffer:      lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
		LineNumber:       lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Placeholder:      lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Prompt:           lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
		Text:             lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}
	s.Cursor = textarea.CursorStyle{
		Color: lipgloss.Color("15"),
		Shape: tea.CursorBlock,
		Blink: true,
	}
	return s
}

// inputHeight returns the number of lines the input area occupies.
func (m tuiModel) inputHeight() int {
	// textarea height + hub line + status line + blank separator
	return m.input.Height() + 3
}

// formatTokenCount formats token counts with k suffix for readability.
func formatTokenCount(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// renderHub builds the status bar showing model, workspace, and token usage.
func (m tuiModel) renderHub() string {
	usage := m.agent.SessionUsage()
	parts := []string{
		fmt.Sprintf(" ws:%s", m.workspace),
		fmt.Sprintf("model:%s", m.agent.Model()),
		fmt.Sprintf("tok:%s/%s", formatTokenCount(usage.PromptTokens), formatTokenCount(usage.CompletionTokens)),
	}
	if usage.ReasoningTokens > 0 {
		parts = append(parts, fmt.Sprintf("reason:%s", formatTokenCount(usage.ReasoningTokens)))
	}
	if usage.CacheReadTokens > 0 || usage.CacheWriteTokens > 0 {
		parts = append(parts, fmt.Sprintf("cache:%s/%s", formatTokenCount(usage.CacheReadTokens), formatTokenCount(usage.CacheWriteTokens)))
	}
	return m.hubStyle.Render(strings.Join(parts, " │")) + "\n"
}

// renderLastAssistant finds the last assistant message and renders it through glamour.
func (m *tuiModel) renderLastAssistant() {
	if m.renderer == nil {
		return
	}
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].role == "assistant" && m.messages[i].rendered == "" {
			rendered, err := m.renderer.Render(m.messages[i].raw)
			if err == nil {
				m.messages[i].rendered = strings.TrimRight(rendered, "\n")
			}
			return
		}
	}
}

// refreshViewport rebuilds the viewport content from all messages.
func (m *tuiModel) refreshViewport() {
	if !m.ready {
		return
	}

	atBottom := m.viewport.AtBottom() || m.viewport.YOffset() == 0

	content := m.renderAllMessages()
	m.viewport.SetContent(content)

	if atBottom {
		m.viewport.GotoBottom()
	}
}

// renderAllMessages produces the full rendered output for the viewport.
func (m tuiModel) renderAllMessages() string {
	var b strings.Builder
	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			b.WriteString(m.userStyle.Render("> "+msg.raw) + "\n")
		case "assistant":
			if msg.rendered != "" {
				// Use glamour-rendered markdown
				b.WriteString(msg.rendered + "\n")
			} else {
				// Still streaming — show raw text, wrapped to prevent TUI hangs on long lines
				w := m.width - 2
				if w < 10 {
					w = 10
				}
				wrapped := wrap.String(msg.raw, w)
				b.WriteString(wrapped)
				if len(wrapped) > 0 && !strings.HasSuffix(wrapped, "\n") {
					b.WriteString("\n")
				}
			}
		case "tool":
			b.WriteString(m.toolStyle.Render("  "+msg.raw) + "\n")
		case "error":
			b.WriteString(m.errorStyle.Render("Error: "+msg.raw) + "\n")
		case "system":
			b.WriteString(m.statusStyle.Render(msg.raw) + "\n")
		}
	}
	return b.String()
}
