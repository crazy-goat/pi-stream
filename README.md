# pi-stream

A small streaming proxy for `pi` in RPC mode. It launches `pi --mode rpc`,
sends a single prompt over its JSON-RPC stdin, and renders the event stream
from pi's stdout as styled terminal output:

- thinking tokens in dim italic
- assistant text plain
- tool calls (`🔧 name args`) when the model decides to invoke a tool
- tool execution (`⚡ name: cmd`) when the tool actually runs
- truncated tool result with `✓` / `✗` status

## Requirements

- A working `pi` binary on `$PATH` (pi-stream spawns it as a subprocess)

## Install

**One-liner** — Linux and macOS, detects OS/arch automatically, installs to `~/.local/bin`:

```sh
curl -sSfL https://raw.githubusercontent.com/crazy-goat/pi-stream/main/install.sh | sh
```

Custom install directory:

```sh
curl -sSfL https://raw.githubusercontent.com/crazy-goat/pi-stream/main/install.sh | INSTALL_DIR=/usr/local/bin sh
```

**From source** (requires Go 1.23+):

```sh
make install        # builds and copies pi-stream to ~/.local/bin
```

## Usage

```sh
pi-stream [flags] <prompt>
```

### Flags

| Flag         | Default | Description                                                             |
| ------------ | ------- | ----------------------------------------------------------------------- |
| `--model`    | (auto)  | Model name forwarded to pi (e.g. `"GLM 5.1"`).                          |
| `--thinking` | `high`  | Thinking level: `off`, `minimal`, `low`, `medium`, `high`, `xhigh`.    |
| `-t`         | (none)  | Comma-separated tool allowlist (e.g. `bash,read`).                      |
| `--session`  | (none)  | pi session file path; share between invocations to carry context over.  |
| `--version`  |         | Print version and exit.                                                 |

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

| Code | Meaning                                          |
| ---- | ------------------------------------------------ |
| 0    | Normal completion (`agent_end` received)         |
| 1    | pi reported an error, or startup failed          |
| 2    | Invalid CLI flags / missing prompt               |
| 130  | Interrupted by `SIGINT` or `SIGTERM` (Ctrl+C)   |

## Development

```sh
make build        # compile
make test         # go test -race ./...
make lint         # golangci-lint run
make tidy         # go mod tidy + diff check
```

Layout:

```
main.go                  # thin entrypoint — signal handling + os.Exit
internal/event/          # typed structs for pi's JSON-RPC event stream
internal/render/         # state-machine Renderer: styles events as ANSI output
internal/pi/             # subprocess lifecycle (Start, Events, Close)
internal/cli/            # flag parsing + event-loop orchestration
```

## License

MIT
