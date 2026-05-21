// Command pi-stream is a streaming proxy for `pi --mode rpc`. It launches
// pi as a subprocess, forwards a single user prompt over its JSON-RPC
// stdin, and renders the event stream from pi's stdout as styled
// terminal output (thinking in dim italic, tool calls, tool execution
// results, plain text).
//
// All real work lives under internal/. This file only wires up signal
// handling and process exit.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/crazy-goat/pi-stream/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
