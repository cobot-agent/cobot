package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/agent"
	cobot "github.com/cobot-agent/cobot/pkg"
)

const maxResultDisplayLen = 200

// chatMessage holds both raw and rendered forms of a message.
type chatMessage struct {
	role     string // "user", "assistant", "tool", "error", "system"
	raw      string // raw content (markdown for assistant, plain for others)
	rendered string // glamour-rendered content (only for assistant messages)
}

type tuiModel struct {
	input        textinput.Model
	viewport     viewport.Model
	messages     []chatMessage
	agent        *agent.Agent
	streaming    bool
	streamCh     <-chan cobot.Event
	streamCancel context.CancelFunc
	pending      []string
	renderer     *glamour.TermRenderer
	width        int
	height       int
	ready        bool // viewport initialized
}

type streamMsg struct {
	content   string
	eventType cobot.EventType
	toolName  string
	done      bool
	err       error
}

var (
	userStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#87CEEB")).Bold(true)
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	toolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Faint(true)
	statusStyle = lipgloss.NewStyle().Faint(true)
	queuedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Faint(true)
)

func newGlamourRenderer(width int) *glamour.TermRenderer {
	w := width - 2 // leave some margin
	if w < 40 {
		w = 40
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(w),
	)
	if err != nil {
		// Fallback: no markdown rendering if glamour fails
		return nil
	}
	return r
}

func newTUIModel(a *agent.Agent) tuiModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()
	ti.CharLimit = 4096
	return tuiModel{
		input:    ti,
		agent:    a,
		messages: []chatMessage{},
	}
}

func (m tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

// inputHeight returns the number of lines the input area occupies.
func (m tuiModel) inputHeight() int {
	// input line + status line + blank separator
	return 3
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.renderer = newGlamourRenderer(m.width)

		vpHeight := m.height - m.inputHeight()
		if vpHeight < 1 {
			vpHeight = 1
		}

		if !m.ready {
			m.viewport = viewport.New(m.width, vpHeight)
			m.viewport.SetContent(m.renderAllMessages())
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = vpHeight
			m.viewport.SetContent(m.renderAllMessages())
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.streaming && m.streamCancel != nil {
				m.streamCancel()
				m.streaming = false
				m.messages = append(m.messages, chatMessage{role: "system", raw: "(cancelled)"})
				m.refreshViewport()
				return m, m.drainPending()
			}
			return m, tea.Quit
		case tea.KeyEnter:
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			if text == "/quit" || text == "/exit" {
				return m, tea.Quit
			}
			m.input.SetValue("")
			if m.streaming {
				m.pending = append(m.pending, text)
				m.messages = append(m.messages, chatMessage{
					role: "user",
					raw:  text + " (queued)",
				})
				m.refreshViewport()
				return m, nil
			}
			return m, m.startStream(text)
		}

		// Pass key events to viewport for scrolling (↑↓ PgUp PgDn)
		if m.ready {
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			cmds = append(cmds, vpCmd)
		}

	case streamMsg:
		mdl, cmd := m.handleStreamMsg(msg)
		return mdl, cmd
	}

	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	cmds = append(cmds, inputCmd)

	return m, tea.Batch(cmds...)
}

func (m *tuiModel) startStream(text string) tea.Cmd {
	m.messages = append(m.messages, chatMessage{role: "user", raw: text})
	m.streaming = true
	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel
	ch, err := m.agent.Stream(ctx, text)
	if err != nil {
		cancel()
		return func() tea.Msg { return streamMsg{err: err} }
	}
	m.streamCh = ch
	m.refreshViewport()
	return m.readNextEvent()
}

func (m *tuiModel) drainPending() tea.Cmd {
	if len(m.pending) == 0 {
		return nil
	}
	next := m.pending[0]
	m.pending = m.pending[1:]
	return m.startStream(next)
}

func (m tuiModel) readNextEvent() tea.Cmd {
	ch := m.streamCh
	return func() tea.Msg {
		evt, ok := <-ch
		if !ok {
			return streamMsg{done: true}
		}
		sm := streamMsg{
			content:   evt.Content,
			eventType: evt.Type,
			done:      evt.Done,
			err:       evt.Error,
		}
		if evt.ToolCall != nil {
			sm.toolName = evt.ToolCall.Name
		}
		return sm
	}
}

func (m tuiModel) handleStreamMsg(msg streamMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.streaming = false
		m.streamCancel = nil
		m.streamCh = nil
		m.messages = append(m.messages, chatMessage{role: "error", raw: msg.err.Error()})
		m.renderLastAssistant()
		m.refreshViewport()
		return m, m.drainPending()
	}
	if msg.done {
		m.streaming = false
		m.streamCancel = nil
		m.streamCh = nil
		m.renderLastAssistant()
		m.refreshViewport()
		return m, m.drainPending()
	}

	switch msg.eventType {
	case cobot.EventText:
		// Accumulate raw text into the current assistant message
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].role == "assistant" {
			m.messages[len(m.messages)-1].raw += msg.content
		} else {
			m.messages = append(m.messages, chatMessage{role: "assistant", raw: msg.content})
		}
	case cobot.EventToolCall:
		m.messages = append(m.messages, chatMessage{role: "tool", raw: fmt.Sprintf("[Tool: %s]", msg.toolName)})
	case cobot.EventToolResult:
		short := msg.content
		if len(short) > maxResultDisplayLen {
			short = short[:maxResultDisplayLen] + "..."
		}
		m.messages = append(m.messages, chatMessage{role: "tool", raw: fmt.Sprintf("[Result: %s]", short)})
	case cobot.EventError:
		m.messages = append(m.messages, chatMessage{role: "error", raw: msg.content})
	}

	m.refreshViewport()
	return m, m.readNextEvent()
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

// refreshViewport rebuilds the viewport content from all messages and scrolls to bottom.
func (m *tuiModel) refreshViewport() {
	if !m.ready {
		return
	}
	content := m.renderAllMessages()
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

// renderAllMessages produces the full rendered output for the viewport.
func (m tuiModel) renderAllMessages() string {
	var b strings.Builder
	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			b.WriteString(userStyle.Render("> "+msg.raw) + "\n")
		case "assistant":
			if msg.rendered != "" {
				// Use glamour-rendered markdown
				b.WriteString(msg.rendered + "\n")
			} else {
				// Still streaming — show raw text
				b.WriteString(msg.raw)
			}
		case "tool":
			b.WriteString(toolStyle.Render("  "+msg.raw) + "\n")
		case "error":
			b.WriteString(errorStyle.Render("Error: "+msg.raw) + "\n")
		case "system":
			b.WriteString(statusStyle.Render(msg.raw) + "\n")
		}
	}
	return b.String()
}

func (m tuiModel) View() string {
	if !m.ready {
		return "Initializing...\n"
	}

	var b strings.Builder

	// Viewport (scrollable message area)
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Status line
	if m.streaming {
		status := statusStyle.Render("Thinking...")
		if len(m.pending) > 0 {
			status += statusStyle.Render(fmt.Sprintf(" (%d queued)", len(m.pending)))
		}
		b.WriteString(status)
	}
	b.WriteString("\n")

	// Input
	b.WriteString(m.input.View())

	return b.String()
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Start interactive TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		a, _, cleanup, err := initAgent(cfg, false)
		if err != nil {
			return err
		}
		defer cleanup()

		p := tea.NewProgram(newTUIModel(a), tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
