package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Console(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("console", flag.ExitOnError)
	errorsOnly := fs.Bool("errors-only", false, "only show errors and exceptions")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools console [--errors-only]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	tabCtx := MustTab(MustSession(ctx, variant, port))

	type entry struct {
		kind string
		text string
	}

	var (
		mu      sync.Mutex
		entries []entry
	)

	chromedp.ListenTarget(tabCtx, func(ev any) {
		switch e := ev.(type) {
		case *cdpruntime.EventConsoleAPICalled:
			if *errorsOnly && e.Type != cdpruntime.APITypeError {
				return
			}
			parts := make([]string, len(e.Args))
			for i, arg := range e.Args {
				parts[i] = formatRemoteObject(arg)
			}
			mu.Lock()
			entries = append(entries, entry{strings.ToUpper(string(e.Type)), strings.Join(parts, " ")})
			mu.Unlock()
		case *cdpruntime.EventExceptionThrown:
			mu.Lock()
			entries = append(entries, entry{"EXCEPTION", e.ExceptionDetails.Error()})
			mu.Unlock()
		}
	})

	// Chrome replays its stored console messages on every Runtime.enable. The
	// attach that picked this tab already enabled Runtime before the listener
	// existed, so it is toggled off and on to get the replay here.
	if err := RunBounded(tabCtx, cdpruntime.Disable(), cdpruntime.Enable()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// The replayed events arrive before the Runtime.enable response; give the
	// listener goroutine a moment to finish processing them all.
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(entries) == 0 {
		fmt.Println("No console messages.")
		return
	}

	for _, e := range entries {
		fmt.Printf("[%s] %s\n", e.kind, e.text)
	}
}

func formatRemoteObject(obj *cdpruntime.RemoteObject) string {
	if obj.Description != "" {
		return obj.Description
	}
	if len(obj.Value) > 0 {
		var s string
		if json.Unmarshal(obj.Value, &s) == nil {
			return s
		}
		return string(obj.Value)
	}
	return string(obj.Type)
}
