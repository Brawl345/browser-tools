package cmd

import (
	"context"
	"fmt"
	"os"

	"browser-tools/browser"

	flag "github.com/spf13/pflag"
)

// fakeMediaArgs give getUserMedia a synthetic camera and microphone and grant
// the permission prompt, which is browser UI that CDP cannot click.
var fakeMediaArgs = []string{
	"--use-fake-device-for-media-stream",
	"--use-fake-ui-for-media-stream",
}

func Start(variant string, port int, args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	fakeMedia := fs.Bool("fake-media", false, "provide a synthetic camera and microphone and grant media permissions without a prompt")
	extra := fs.StringArray("arg", nil, "extra Chrome command-line flag, repeatable (e.g. --arg=--force-dark-mode)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools start [--fake-media] [--arg FLAG]...")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Starts the browser with remote debugging if not already running")
		fmt.Fprintln(os.Stderr, "and verifies the connection. Launch flags need a fresh start;")
		fmt.Fprintln(os.Stderr, "with a browser already on the port, start fails instead.")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	launchArgs := append([]string{}, *extra...)
	if *fakeMedia {
		launchArgs = append(launchArgs, fakeMediaArgs...)
	}

	if browser.IsRunning(port) {
		if len(launchArgs) > 0 {
			fail("a browser is already running on port %d and ignores launch flags\n  quit it first, then run start again", port)
		}
		fmt.Fprintf(os.Stderr, "already running on port %d\n", port)
		return
	}

	if err := browser.Launch(variant, port, launchArgs...); err != nil {
		fail("%v", err)
	}

	s, err := browser.Open(context.Background(), variant, port)
	if err != nil {
		fail("%v", err)
	}
	if _, err := s.Targets(); err != nil {
		fail("the browser started but does not answer on port %d: %v", port, err)
	}

	fmt.Printf("%s✓%s browser ready on port %d\n", ansiGreen, ansiReset, port)
}
