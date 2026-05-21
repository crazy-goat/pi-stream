# pi-stream

A small streaming proxy for [`pi`](https://github.com/) in RPC mode. It launches
`pi --mode rpc`, sends a single prompt over its JSON-RPC stdin, and renders the
event stream from pi's stdout as styled terminal output:

- thinking tokens in dim italic
- assistant text plain
- tool calls (`🔧 name args`) when the model decides to invoke a tool
- tool execution (`⚡ name: cmd`) when the tool actually runs
- truncated tool result with `✓` / `✗` status

## Requirements

- Go 1.23+ to build
- A working `pi` binary on `$PATH` (this proxy spawns it)

## Install

```sh
make install        # builds and copies pi-stream into ~/.local/bin
# or
go install github.com/crazy-goat/pi-stream@latest
```

## Usage

```sh
pi-stream [flags] <prompt>
```

### Flags

| Flag         | Default | Description                                                  |
| ------------ | ------- | ------------------------------------------------------------ |
| `--model`    | (auto)  | Model name forwarded to pi (e.g. `"GLM 5.1"`).               |
| `--thinking` | `high`  | Thinking level: `off`, `minimal`, `low`, `medium`, `high`, `xhigh`. |
| `-t`         | (none)  | Comma-separated tool allowlist (e.g. `bash,read`).           |
| `--session`  | (none)  | pi session file path; share between invocations for context. |
| `--version`  |         | Print version and exit.                                      |

### Examples

```sh
# Quick one-shot prompt
pi-stream --model "GLM 5.1" --thinking off "tell me a one-line joke"

# Let the model use bash
pi-stream --model "GLM 5.1" -t bash "list files in this repo and summarize"

# Reuse a session across multiple calls
pi-stream --session /tmp/sess "first message"
pi-stream --session /tmp/sess "follow-up that should remember the first"
```

### Exit codes

| Code | Meaning                                            |
| ---- | -------------------------------------------------- |
| 0    | Normal completion (`agent_end` received)           |
| 1    | pi reported an error envelope, or startup failed   |
| 2    | Invalid CLI flags / missing prompt                 |
| 130  | Interrupted by `SIGINT` or `SIGTERM` (Ctrl+C)      |

## Development

```sh
make build        # compile
make test         # go test -race ./...
make lint         # golangci-lint run
make tidy         # go mod tidy + diff check
```

Layout:

```
main.go                       # ~10-line entrypoint
internal/event/               # typed event structs for pi's JSON stream
internal/render/              # state-machine Renderer that styles events
internal/pi/                  # subprocess lifecycle (Start, Events, Close)
internal/cli/                 # flag parsing + event-loop orchestration
```

## License

MIT
