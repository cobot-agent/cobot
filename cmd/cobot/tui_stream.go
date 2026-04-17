package main

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func (m *tuiModel) startStream(text string) tea.Cmd {
	m.messages = append(m.messages, chatMessage{role: "user", raw: text})
	m.streaming = true
	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel
	ch, err := m.agent.Stream(ctx, text)
	if err != nil {
		cancel()
		return func() tea.Msg { return streamMsg{err: err.Error()} }
	}
	m.streamCh = ch
	m.viewport.GotoBottom()
	m.refreshViewport()
	return tea.Batch(m.readNextEvent(), m.spinner.Tick, scheduleRefresh())
}

func (m *tuiModel) finishStream() {
	m.streaming = false
	if m.streamCancel != nil {
		m.streamCancel()
		m.streamCancel = nil
	}
	m.streamCh = nil
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
	// Tool errors (EventToolResult with Error) are non-fatal — the agent loop
	// feeds them back to the model for retry. Only EventError is terminal.
	if msg.err != "" && msg.eventType != cobot.EventToolResult {
		m.finishStream()
		m.messages = append(m.messages, chatMessage{role: "error", raw: msg.err})
		m.renderLastAssistant()
		m.refreshViewport()
		cmd := m.drainPending()
		return m, cmd
	}
	if msg.done {
		m.finishStream()
		m.renderLastAssistant()
		m.refreshViewport()
		cmd := m.drainPending()
		return m, cmd
	}

	switch msg.eventType {
	case cobot.EventText:
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
		prefix := "Result"
		if msg.err != "" {
			prefix = "Error"
		}
		m.messages = append(m.messages, chatMessage{role: "tool", raw: fmt.Sprintf("[%s: %s]", prefix, short)})
	case cobot.EventError:
		m.messages = append(m.messages, chatMessage{role: "error", raw: msg.content})
	}

	m.renderDirty = true
	return m, m.readNextEvent()
}
