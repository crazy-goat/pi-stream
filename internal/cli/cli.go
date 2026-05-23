// Package cli wires the CLI flag set, the pi subprocess, and the renderer
// together. It is the small "orchestrator" layer that main() defers to.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/crazy-goat/pi-stream/internal/event"
	"github.com/crazy-goat/pi-stream/internal/pi"
	"github.com/crazy-goat/pi-stream/internal/render"
)

// Version is the human-readable build version, populated via -ldflags
// "-X github.com/crazy-goat/pi-stream/internal/cli.Version=...".
var Version = "dev"

// Exit codes returned by Run.
const (
	ExitOK        = 0
	ExitError     = 1
	ExitUsage     = 2
	ExitInterrupt = 130
)

// validThinking is the set of accepted --thinking flag values.
var validThinking = map[string]bool{
	"off": true, "minimal": true, "low": true,
	"medium": true, "high": true, "xhigh": true,
}

// Run parses args (excluding the program name), launches pi, and streams
// rendered output to stdout. Diagnostics go to stderr. The returned int
// is the process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pi-stream", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		model    = fs.String("model", "", `Model to use (e.g. "GLM 5.1")`)
		thinking = fs.String("thinking", "high", "Thinking level (off|minimal|low|medium|high|xhigh)")
		tools    = fs.String("t", "", "Comma-separated tool allowlist")
		session  = fs.String("session", "", "pi session file path (shared between steps for context)")
		version  = fs.Bool("version", false, "Print version and exit")
	)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "usage: pi-stream [flags] <prompt>")
		_, _ = fmt.Fprintln(stderr)
		_, _ = fmt.Fprintln(stderr, "Flags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK
		}
		return ExitUsage
	}

	if !validThinking[*thinking] {
		_, _ = fmt.Fprintf(stderr, "invalid --thinking value %q; valid: off, minimal, low, medium, high, xhigh\n", *thinking)
		return ExitUsage
	}

	if *version {
		_, _ = fmt.Fprintln(stdout, Version)
		return ExitOK
	}

	rest := fs.Args()
	if len(rest) == 0 {
		fs.Usage()
		return ExitUsage
	}
	prompt := strings.Join(rest, " ")

	proc, err := pi.Start(ctx, pi.Options{
		Model:    *model,
		Thinking: *thinking,
		Tools:    *tools,
		Session:  *session,
	}, prompt, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "pi-stream: %v\n", err)
		return ExitError
	}

	exit := streamEvents(ctx, proc, stdout, stderr)
	_ = proc.Close()
	return exit
}

// eventSource is the subset of pi.Process that streamEvents needs,
// extracted as an interface for testability.
type eventSource interface {
	Events() (<-chan []byte, <-chan error)
	Kill() error
}

// streamEvents drains the process event channel until completion, an
// error envelope, a scanner error, or context cancellation.
func streamEvents(ctx context.Context, proc eventSource, stdout, stderr io.Writer) int {
	r := render.New(stdout)
	events, errCh := proc.Events()
	for {
		select {
		case <-ctx.Done():
			_ = proc.Kill()
			return ExitInterrupt
		case err := <-errCh:
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "pi-stream: %v\n", err)
				return ExitError
			}
			return ExitOK
		case line, ok := <-events:
			if !ok {
				return ExitOK
			}
			var env event.Envelope
			if err := json.Unmarshal(line, &env); err != nil {
				continue
			}
			done, code := handleEvent(r, env, stderr)
			if done {
				return code
			}
		}
	}
}

// handleEvent processes a single envelope. It returns (true, code) when
// the stream should terminate (agent_end, fatal response/error envelope).
func handleEvent(r *render.Renderer, env event.Envelope, stderr io.Writer) (bool, int) {
	switch env.Type {
	case event.TypeResponse:
		if env.Success != nil && !*env.Success {
			_, _ = fmt.Fprintf(stderr, "pi error: %s\n", env.Error)
			return true, ExitError
		}
	case event.TypeMessageUpdate:
		if env.AssistantMessageEvent != nil {
			handleMessage(r, env.AssistantMessageEvent)
		}
	case event.TypeToolExecStart:
		r.ToolExecStart(env.ToolCallID, env.ToolName, env.Args)
	case event.TypeToolExecUpdate:
		// PartialResult can be nil; skip the call since there is nothing
		// to stream. ToolExecEnd must always run (it closes the box).
		if env.PartialResult != nil {
			r.ToolExecUpdate(env.ToolCallID, env.PartialResult.SummaryText())
		}
	case event.TypeToolExecEnd:
		var summary string
		if env.Result != nil {
			summary = env.Result.SummaryText()
		}
		r.ToolExecEnd(env.ToolCallID, env.IsError, summary)
	case event.TypeTurnStart:
		r.TurnStart()
	case event.TypeTurnEnd:
		r.TurnEnd()
	case event.TypeAgentEnd:
		r.AgentEnd()
		return true, ExitOK
	case event.TypeError:
		msg := env.Error
		if env.AssistantMessageEvent != nil && env.AssistantMessageEvent.Error != "" {
			msg = env.AssistantMessageEvent.Error
		}
		_, _ = fmt.Fprintf(stderr, "pi error: %s\n", msg)
		return true, ExitError
	}
	return false, ExitOK
}

func handleMessage(r *render.Renderer, msg *event.AssistantMessageEvent) {
	switch msg.Type {
	case event.MsgTypeThinkingDelta, event.MsgTypeThinksDelta:
		r.Thinking(msg.Delta)
	case event.MsgTypeTextDelta:
		r.Text(msg.Delta)
		// thinking_start, text_start, toolcall_*: no-op. The tool-execution
		// box (rendered from tool_execution_* events) is enough; rendering
		// toolcall_end would just duplicate the header.
	}
}
