package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/cobot-agent/cobot/internal/agent"
	cobot "github.com/cobot-agent/cobot/pkg"
)

const maxResultDisplayLen = 200

type tuiModel struct {
	input        textinput.Model
	messages     []string
	agent        *agent.Agent
	streaming    bool
	streamCh     <-chan cobot.Event
	streamCancel context.CancelFunc
	width        int
	height       int
}

type streamMsg struct {
	content   string
	eventType cobot.EventType
	toolName  string
	done      bool
	err       error
}

func newTUIModel(a *agent.Agent) tuiModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()
	ti.CharLimit = 4096
	return tuiModel{
		input:    ti,
		agent:    a,
		messages: []string{},
	}
}

func (m tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.streaming && m.streamCancel != nil {
				m.streamCancel()
				m.streaming = false
				m.messages = append(m.messages, "")
				return m, nil
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
			m.messages = append(m.messages, fmt.Sprintf("> %s", text))
			m.input.SetValue("")
			m.streaming = true
			ctx, cancel := context.WithCancel(context.Background())
			m.streamCancel = cancel
			ch, err := m.agent.Stream(ctx, text)
			if err != nil {
				cancel()
				return m, func() tea.Msg { return streamMsg{err: err} }
			}
			m.streamCh = ch
			return m, m.readNextEvent()
		}

	case streamMsg:
		return m.handleStreamMsg(msg)
	}

	if m.streaming {
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
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
		m.messages = append(m.messages, fmt.Sprintf("Error: %v", msg.err))
		return m, nil
	}
	if msg.done {
		m.streaming = false
		m.streamCancel = nil
		m.streamCh = nil
		m.messages = append(m.messages, "")
		return m, nil
	}

	switch msg.eventType {
	case cobot.EventText:
		if len(m.messages) > 0 {
			last := m.messages[len(m.messages)-1]
			if strings.HasPrefix(last, "Assistant:") {
				m.messages[len(m.messages)-1] += msg.content
			} else {
				m.messages = append(m.messages, "Assistant: "+msg.content)
			}
		} else {
			m.messages = append(m.messages, "Assistant: "+msg.content)
		}
	case cobot.EventToolCall:
		m.messages = append(m.messages, fmt.Sprintf("  [Tool: %s]", msg.toolName))
	case cobot.EventToolResult:
		short := msg.content
		if len(short) > maxResultDisplayLen {
			short = short[:maxResultDisplayLen] + "..."
		}
		m.messages = append(m.messages, fmt.Sprintf("  [Result: %s]", short))
	case cobot.EventError:
		m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render("Error: "+msg.content))
	}

	return m, m.readNextEvent()
}

func (m tuiModel) View() string {
	var b strings.Builder
	for _, msg := range m.messages {
		b.WriteString(msg)
		b.WriteString("\n")
	}
	if m.streaming {
		b.WriteString(lipgloss.NewStyle().Faint(true).Render("Thinking..."))
		b.WriteString("\n")
	}
	b.WriteString("\n")
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
