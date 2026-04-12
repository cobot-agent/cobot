package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// MemoryAction represents a single memory curation action
type MemoryAction struct {
	Type       string `json:"type"`
	Content    string `json:"content,omitempty"`
	Wing       string `json:"wing,omitempty"`
	Room       string `json:"room,omitempty"`
	ID         string `json:"id,omitempty"`
	NewContent string `json:"new_content,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// MemoryActionResult represents the parsed result from LLM
type MemoryActionResult struct {
	Actions []MemoryAction `json:"actions"`
}

// Agent is the intelligent memory curator that runs in background
type Agent struct {
	provider    cobot.Provider
	memoryStore Client
	session     *SessionTracker
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	interval    time.Duration
	minMessages int
}

// SessionTracker tracks conversation for memory agent
type SessionTracker struct {
	mu       sync.RWMutex
	messages []cobot.Message
}

func NewSessionTracker() *SessionTracker {
	return &SessionTracker{
		messages: make([]cobot.Message, 0),
	}
}

func (st *SessionTracker) AddMessage(msg cobot.Message) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.messages = append(st.messages, msg)
}

func (st *SessionTracker) RecentMessages(n int) []cobot.Message {
	st.mu.RLock()
	defer st.mu.RUnlock()

	if len(st.messages) <= n {
		out := make([]cobot.Message, len(st.messages))
		copy(out, st.messages)
		return out
	}

	start := len(st.messages) - n
	out := make([]cobot.Message, n)
	copy(out, st.messages[start:])
	return out
}

func (st *SessionTracker) AllMessages() []cobot.Message {
	st.mu.RLock()
	defer st.mu.RUnlock()
	out := make([]cobot.Message, len(st.messages))
	copy(out, st.messages)
	return out
}

func (st *SessionTracker) Reset() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.messages = st.messages[:0]
}

// NewAgent creates a new Memory Agent
func NewAgent(provider cobot.Provider, store Client) *Agent {
	ctx, cancel := context.WithCancel(context.Background())
	return &Agent{
		provider:    provider,
		memoryStore: store,
		session:     NewSessionTracker(),
		ctx:         ctx,
		cancel:      cancel,
		interval:    30 * time.Second,
		minMessages: 3,
	}
}

// Start begins the background memory curation
func (ma *Agent) Start() {
	ma.wg.Add(1)
	go ma.run()
	slog.Info("memory-agent: started")
}

// Stop gracefully shuts down the memory agent
func (ma *Agent) Stop() {
	ma.cancel()
	ma.wg.Wait()
	slog.Info("memory-agent: stopped")
}

// AddMessage notifies the memory agent of a new message
func (ma *Agent) AddMessage(msg cobot.Message) {
	ma.session.AddMessage(msg)
}

func (ma *Agent) run() {
	defer ma.wg.Done()

	ticker := time.NewTicker(ma.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ma.analyzeAndCurate()
		case <-ma.ctx.Done():
			ma.analyzeAndCurate()
			return
		}
	}
}

func (ma *Agent) analyzeAndCurate() {
	messages := ma.session.RecentMessages(20)
	if len(messages) < ma.minMessages {
		return
	}

	hasContent := false
	for _, msg := range messages {
		if msg.Role == cobot.RoleUser || msg.Role == cobot.RoleAssistant {
			if len(msg.Content) > 10 {
				hasContent = true
				break
			}
		}
	}
	if !hasContent {
		return
	}

	conversation := ma.formatConversation(messages)
	relevantMemories := ma.findRelevantMemories(messages)
	prompt := buildCurationPrompt(conversation, relevantMemories)

	callCtx, cancel := context.WithTimeout(ma.ctx, 15*time.Second)
	defer cancel()

	resp, err := ma.provider.Complete(callCtx, &cobot.ProviderRequest{
		Model: "gpt-4o-mini",
		Messages: []cobot.Message{{
			Role:    cobot.RoleUser,
			Content: prompt,
		}},
		MaxTokens: 2000,
	})
	if err != nil {
		slog.Debug("memory-agent: curation failed", "error", err)
		return
	}

	actions := ma.parseActions(resp.Content)
	for _, action := range actions {
		ma.executeAction(action)
	}

	if len(actions) > 0 {
		slog.Info("memory-agent: executed actions", "count", len(actions))
	}
}

func (ma *Agent) formatConversation(messages []cobot.Message) string {
	var b strings.Builder
	for _, msg := range messages {
		switch msg.Role {
		case cobot.RoleUser:
			b.WriteString("User: ")
		case cobot.RoleAssistant:
			b.WriteString("Assistant: ")
		default:
			continue
		}
		b.WriteString(msg.Content)
		b.WriteString("\n\n")
	}
	return b.String()
}

func (ma *Agent) findRelevantMemories(messages []cobot.Message) string {
	keywords := ma.extractKeywords(messages)
	if len(keywords) == 0 {
		return "No existing relevant memories."
	}

	var allResults []*cobot.SearchResult
	for _, kw := range keywords {
		if len(kw) < 3 {
			continue
		}
		results, err := ma.memoryStore.Search(ma.ctx, &cobot.SearchQuery{
			Text:  kw,
			Limit: 3,
		})
		if err == nil {
			allResults = append(allResults, results...)
		}
	}

	if len(allResults) == 0 {
		return "No existing relevant memories."
	}

	seen := make(map[string]bool)
	var b strings.Builder
	for _, r := range allResults {
		if seen[r.DrawerID] {
			continue
		}
		seen[r.DrawerID] = true
		b.WriteString(fmt.Sprintf("- [%s] %s\n", r.DrawerID, truncate(r.Content, 100)))
	}
	return b.String()
}

func (ma *Agent) extractKeywords(messages []cobot.Message) []string {
	var keywords []string
	for _, msg := range messages {
		if msg.Role != cobot.RoleUser {
			continue
		}
		words := strings.Fields(msg.Content)
		for _, w := range words {
			w = strings.TrimRight(w, ".,!?;:()")
			if len(w) >= 4 {
				keywords = append(keywords, w)
			}
		}
	}
	return keywords
}

func (ma *Agent) parseActions(content string) []MemoryAction {
	jsonStr := content
	if idx := strings.Index(content, "```json"); idx != -1 {
		start := idx + 7
		if end := strings.Index(content[start:], "```"); end != -1 {
			jsonStr = content[start : start+end]
		}
	} else if idx := strings.Index(content, "{"); idx != -1 {
		if end := strings.LastIndex(content, "}"); end != -1 && end > idx {
			jsonStr = content[idx : end+1]
		}
	}

	jsonStr = strings.TrimSpace(jsonStr)

	var result MemoryActionResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		slog.Debug("memory-agent: failed to parse actions", "error", err, "content", truncate(content, 200))
		return nil
	}

	return result.Actions
}

func (ma *Agent) executeAction(action MemoryAction) {
	switch action.Type {
	case "store":
		if action.Content == "" || action.Wing == "" || action.Room == "" {
			return
		}
		existing, _ := ma.memoryStore.Search(ma.ctx, &cobot.SearchQuery{
			Text:  action.Content,
			Limit: 1,
		})
		if len(existing) > 0 && existing[0].Score > 0.95 {
			slog.Debug("memory-agent: skipping duplicate", "content", truncate(action.Content, 50))
			return
		}

		wingID, _ := ma.memoryStore.CreateWingIfNotExists(ma.ctx, action.Wing)
		roomID, _ := ma.memoryStore.CreateRoomIfNotExists(ma.ctx, wingID, action.Room, "facts")
		_, err := ma.memoryStore.Store(ma.ctx, action.Content, wingID, roomID)
		if err != nil {
			slog.Debug("memory-agent: store failed", "error", err)
		} else {
			slog.Info("memory-agent: stored", "wing", action.Wing, "room", action.Room, "reason", action.Reason)
		}

	case "update":
		if action.ID != "" && action.NewContent != "" {
			slog.Debug("memory-agent: update not implemented", "id", action.ID)
		}

	case "delete":
		if action.ID != "" {
			slog.Debug("memory-agent: delete not implemented", "id", action.ID)
		}
	}
}

func buildCurationPrompt(conversation, relevantMemories string) string {
	return fmt.Sprintf(`You are a Memory Curator AI. Your job is to analyze conversations and manage long-term memory efficiently.

## Current Conversation
%s

## Existing Relevant Memories
%s

## Your Task
Analyze the conversation and decide what memory actions to take. Focus on:
1. **User preferences** (tech stack, habits, likes/dislikes)
2. **Important decisions** ("let's use X", "I choose Y")
3. **Key facts** (project details, requirements, constraints)
4. **Updates to existing memories** (if user changes preference)

## Rules
- Only store significant, long-term valuable information
- Don't store temporary or context-specific details
- Merge similar information rather than creating duplicates
- Use simple, clear wing/room names (e.g., wing="preferences", room="tech-stack")

## Output Format
Return a JSON object with actions:
`+"`"+`json
{
  "actions": [
    {"type": "store", "content": "...", "wing": "...", "room": "...", "reason": "..."},
    {"type": "update", "id": "...", "new_content": "...", "reason": "..."},
    {"type": "delete", "id": "...", "reason": "..."}
  ]
}
`+"`"+`

If no action needed, return {"actions": []}.`, conversation, relevantMemories)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
