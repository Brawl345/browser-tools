package cmd

import (
	"context"
	"fmt"
	"os"

	"browser-tools/browser"
	flag "github.com/spf13/pflag"
)

func Start(variant string, port int, args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools start")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Starts the browser with remote debugging if not already running")
		fmt.Fprintln(os.Stderr, "and verifies the connection.")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if browser.IsRunning(port) {
		fmt.Fprintf(os.Stderr, "already running on port %d\n", port)
		return
	}

	if err := browser.Launch(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Verify connection.
	allocCtx, _ := browser.Connect(context.Background(), port)
	_, _ = browser.NewTab(allocCtx)

	fmt.Printf("%s✓%s browser ready on port %d\n", ansiGreen, ansiReset, port)
}
