package cobot

import "context"

// CommandContext holds everything a command needs to execute.
type CommandContext struct {
	Platform string // "feishu", "weixin", "reverse"
	ChatID   string
	UserID   string
	Text     string // command arguments (name and args already split by registry)

	// Agent is the agent instance. Commands can call Agent.SkillManager(),
	// Agent.Provider(), Agent.Registry(). Must be set by bootstrap.
	Agent interface{}

	// Data is an optional bag of extra dependencies. Bootstrap can use this
	// to pass e.g. a *cron.Scheduler or a *Store. Commands type-assert as needed.
	Data interface{}

	// Reply sends a message back through the originating channel.
	Reply func(msg *OutboundMessage) (*SendResult, error)
}

// Command executes a slash command and optionally returns a response.
// If it returns nil, the command is considered handled (e.g. side-effect only).
type Command interface {
	// Name returns the command name without the leading slash, e.g. "skill", "stop".
	Name() string

	// Help returns a short one-line description for the help text.
	Help() string

	// Execute runs the command. ctx may be cancelled if the user disconnects.
	Execute(ctx context.Context, cmdCtx CommandContext) (*OutboundMessage, error)
}

// CommandRegistry is the registry of available slash commands.
// Commands are dispatched by name (e.g. "/skill list" → Command("skill").Execute).
type CommandRegistry interface {
	// Execute finds the command by name (first token without the leading /)
	// and runs it. Returns (handled, error): handled=true means the message
	// was a command and was processed (success or failure); handled=false
	// means the text was not a command and should be passed to the agent.
	Execute(ctx context.Context, cmdCtx CommandContext) (handled bool, err error)

	// Commands returns all registered commands for help generation.
	Commands() []Command

	// Register adds a command. Panics if a command with the same name exists.
	Register(cmd Command)
}

// NewCommandRegistry creates an empty registry.
func NewCommandRegistry() CommandRegistry {
	return &commandRegistry{
		cmds: make(map[string]Command),
	}
}

type commandRegistry struct {
	cmds map[string]Command
}

func (r *commandRegistry) Register(cmd Command) {
	if _, exists := r.cmds[cmd.Name()]; exists {
		panic("command already registered: " + cmd.Name())
	}
	r.cmds[cmd.Name()] = cmd
}

func (r *commandRegistry) Execute(ctx context.Context, cmdCtx CommandContext) (bool, error) {
	name, args := parseCommandName(cmdCtx.Text)
	if name == "" {
		return false, nil
	}
	cmd, ok := r.cmds[name]
	if !ok {
		return false, nil // unknown command → fall through to agent
	}
	// Swap in just the args for the command's Execute.
	cmdCtx.Text = args
	msg, err := cmd.Execute(ctx, cmdCtx)
	if err != nil {
		if msg != nil {
			_, _ = cmdCtx.Reply(msg)
		}
		return true, err
	}
	if msg != nil {
		_, _ = cmdCtx.Reply(msg)
	}
	return true, nil
}

func (r *commandRegistry) Commands() []Command {
	out := make([]Command, 0, len(r.cmds))
	for _, cmd := range r.cmds {
		out = append(out, cmd)
	}
	return out
}

// parseCommandName splits "/skill list arg" into ("skill", "list arg").
// Returns ("", "") if text doesn't start with "/".
func parseCommandName(text string) (name, args string) {
	if len(text) == 0 || text[0] != '/' {
		return "", ""
	}
	rest := text[1:]
	spaceIdx := -1
	for i, c := range rest {
		if c == ' ' || c == '\t' {
			spaceIdx = i
			break
		}
	}
	if spaceIdx < 0 {
		return rest, ""
	}
	return rest[:spaceIdx], rest[spaceIdx+1:]
}
