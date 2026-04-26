package sandbox

import (
	"bytes"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// ShellSegment represents a parsed shell command segment.
type ShellSegment struct {
	// Raw is the full serialized segment (e.g. "curl http://example.com",
	// "$(curl localhost)", "ls -la").
	Raw string
	// BaseCmd is the base name of the first word (e.g. "curl", "ls", "rm").
	// Empty if the segment starts with a command substitution.
	BaseCmd string
	// HasCmdSubst is true if Raw contains a command substitution $(...) or `...`.
	HasCmdSubst bool
	// InnerCmds lists all commands executed inside command substitutions
	// within this segment. For example, "$(curl localhost)" has InnerCmds=["curl"].
	// These are checked against BlockedCommands separately from BaseCmd,
	// since the outer command (e.g. "echo") is not the security-relevant one.
	InnerCmds []string
}

// ParseShellCommand parses a shell command string into structured segments.
// It correctly handles:
//   - Quoted strings: "hello && world" stays as one word
//   - Command substitution: $(curl localhost) is detected and marked
//   - Pipe, semicolon, &&, ||: properly split into separate statements
//   - Background &, newlines: properly split
//
// Each CallExpr (simple command) produces one ShellSegment with its arguments
// grouped together.
func ParseShellCommand(cmd string) ([]ShellSegment, error) {
	r := syntax.NewParser()
	var segments []ShellSegment

	if err := r.Stmts(strings.NewReader(cmd), func(stmt *syntax.Stmt) bool {
		collectSegments(stmt, &segments)
		return true
	}); err != nil {
		return nil, err
	}

	return segments, nil
}

// collectSegments walks a statement and appends its segments.
func collectSegments(stmt *syntax.Stmt, out *[]ShellSegment) {
	if stmt == nil || stmt.Cmd == nil {
		return
	}

	switch cmd := stmt.Cmd.(type) {
	case *syntax.CallExpr:
		// Group all CallExpr args into one segment.
		seg := ShellSegment{}
		var buf bytes.Buffer
		for i, w := range cmd.Args {
			if i > 0 {
				buf.WriteByte(' ')
			}
			wordStr := serializeWord(w)
			buf.WriteString(wordStr)
			if hasCmdSubstitution(w) {
				seg.HasCmdSubst = true
				// Extract commands executed inside $(...) or `...` within this word.
				extractInnerCommands(w, &seg.InnerCmds)
			}
		}
		seg.Raw = buf.String()

		if !seg.HasCmdSubst {
			// No command substitution — BaseCmd is the first argument.
			if len(cmd.Args) > 0 {
				seg.BaseCmd = filepath.Base(cmd.Args[0].Lit())
			}
		} else if len(cmd.Args) > 0 && !hasCmdSubstitution(cmd.Args[0]) {
			// Segment starts with a normal word followed by words with cmd substitution.
			// Example: "echo $(curl localhost)" — BaseCmd="echo", InnerCmds=["curl"].
			seg.BaseCmd = filepath.Base(cmd.Args[0].Lit())
		}
		// If the segment starts with a cmd substitution (e.g. "$(curl localhost)"),
		// BaseCmd remains empty; all commands are in InnerCmds.

		*out = append(*out, seg)

	case *syntax.BinaryCmd:
		// &&, ||, |, ;, >, <, etc.
		collectSegments(cmd.X, out)
		collectSegments(cmd.Y, out)

	case *syntax.Subshell:
		for _, s := range cmd.Stmts {
			collectSegments(s, out)
		}

	case *syntax.WhileClause:
		for _, s := range cmd.Cond {
			collectSegments(s, out)
		}
		for _, s := range cmd.Do {
			collectSegments(s, out)
		}

	case *syntax.ForClause:
		for _, s := range cmd.Do {
			collectSegments(s, out)
		}

	case *syntax.IfClause:
		for _, s := range cmd.Cond {
			collectSegments(s, out)
		}
		for _, s := range cmd.Then {
			collectSegments(s, out)
		}
		if cmd.Else != nil {
			collectSegments(&syntax.Stmt{Cmd: cmd.Else}, out)
		}

	case *syntax.CaseClause:
		if cmd.Word != nil {
			seg := ShellSegment{
				Raw:         serializeWord(cmd.Word),
				HasCmdSubst: hasCmdSubstitution(cmd.Word),
			}
			if !seg.HasCmdSubst {
				seg.BaseCmd = filepath.Base(cmd.Word.Lit())
			}
			*out = append(*out, seg)
		}

	case *syntax.FuncDecl:
		collectSegments(cmd.Body, out)

	default:
		// Fall back: serialize and check for command substitution.
		seg := ShellSegment{
			Raw:         serializeNode(stmt.Cmd),
			HasCmdSubst: hasCmdSubstNode(stmt.Cmd),
		}
		if !seg.HasCmdSubst {
			seg.BaseCmd = filepath.Base(seg.Raw)
		}
		*out = append(*out, seg)
	}
}

// serializeWord serializes a *syntax.Word back to its string form.
func serializeWord(w *syntax.Word) string {
	var buf bytes.Buffer
	syntax.NewPrinter().Print(&buf, w)
	return buf.String()
}

// serializeNode serializes any syntax.Node to its string form.
func serializeNode(n syntax.Node) string {
	var buf bytes.Buffer
	syntax.NewPrinter().Print(&buf, n)
	return buf.String()
}

// hasCmdSubstitution reports whether w contains a command substitution $(...) or `...`.
func hasCmdSubstitution(w *syntax.Word) bool {
	found := false
	syntax.Walk(w, func(node syntax.Node) bool {
		if _, ok := node.(*syntax.CmdSubst); ok {
			found = true
		}
		return !found
	})
	return found
}

// hasCmdSubstNode reports whether n contains any command substitution.
func hasCmdSubstNode(n syntax.Node) bool {
	found := false
	syntax.Walk(n, func(node syntax.Node) bool {
		if _, ok := node.(*syntax.CmdSubst); ok {
			found = true
		}
		return !found
	})
	return found
}

// extractInnerCommands walks node and finds all commands executed inside
// command substitutions, appending their base names to out.
// For example, in $(curl localhost) it finds the CallExpr whose first arg is "curl".
func extractInnerCommands(node syntax.Node, out *[]string) {
	syntax.Walk(node, func(n syntax.Node) bool {
		if cs, ok := n.(*syntax.CmdSubst); ok {
			// Descend into all statements inside the command substitution.
			for _, stmt := range cs.Stmts {
				collectCmds(stmt, out)
			}
			return true // continue walking (in case of nested CmdSubst)
		}
		return true
	})
}

// collectCmds collects all top-level command names from a statement into out.
func collectCmds(stmt *syntax.Stmt, out *[]string) {
	if stmt == nil || stmt.Cmd == nil {
		return
	}
	switch c := stmt.Cmd.(type) {
	case *syntax.CallExpr:
		if len(c.Args) > 0 && c.Args[0].Lit() != "" {
			*out = append(*out, filepath.Base(c.Args[0].Lit()))
		}
	case *syntax.BinaryCmd:
		collectCmds(c.X, out)
		collectCmds(c.Y, out)
	case *syntax.Subshell:
		for _, s := range c.Stmts {
			collectCmds(s, out)
		}
	case *syntax.WhileClause:
		for _, s := range c.Cond {
			collectCmds(s, out)
		}
		for _, s := range c.Do {
			collectCmds(s, out)
		}
	case *syntax.ForClause:
		for _, s := range c.Do {
			collectCmds(s, out)
		}
	case *syntax.IfClause:
		for _, s := range c.Cond {
			collectCmds(s, out)
		}
		for _, s := range c.Then {
			collectCmds(s, out)
		}
		if c.Else != nil {
			collectCmds(&syntax.Stmt{Cmd: c.Else}, out)
		}
	case *syntax.FuncDecl:
		collectCmds(c.Body, out)
	}
	// For any other cmd type, we don't extract (e.g. time, coproc, etc.)
}

// ShellCommandSegments returns the raw text of each shell segment.
// Each CallExpr (simple command) produces one segment with all its arguments.
// This is a string-based API kept for backward compatibility.
func ShellCommandSegments(cmd string) []string {
	segments, err := ParseShellCommand(cmd)
	if err != nil {
		return naiveShellSegments(cmd)
	}
	result := make([]string, 0, len(segments))
	for _, seg := range segments {
		result = append(result, seg.Raw)
	}
	return result
}

// naiveShellSegments is the original naive implementation used as fallback.
func naiveShellSegments(cmd string) []string {
	replacer := strings.NewReplacer(
		"\r\n", "\n",
		"&&", "\n",
		"||", "\n",
		"&", "\n",
		";", "\n",
		"|", "\n",
	)
	return strings.Split(replacer.Replace(cmd), "\n")
}
