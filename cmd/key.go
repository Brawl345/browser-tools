package cmd

import (
	"context"
	"fmt"
	"os"

	"browser-tools/browser"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	flag "github.com/spf13/pflag"
)

func Key(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("key", flag.ExitOnError)
	sel := fs.String("selector", "", "CSS selector to focus before pressing the key")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools key [--selector <sel>] <key>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Common keys: Enter, Escape, Tab, Backspace, Delete,")
		fmt.Fprintln(os.Stderr, "             ArrowLeft, ArrowRight, ArrowUp, ArrowDown")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() == 0 {
		fs.Usage()
		os.Exit(1)
	}

	key := fs.Arg(0)

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	keyNames := map[string]string{
		"Enter":       kb.Enter,
		"Tab":         kb.Tab,
		"Backspace":   kb.Backspace,
		"Escape":      kb.Escape,
		"Delete":      kb.Delete,
		"ArrowLeft":   kb.ArrowLeft,
		"ArrowRight":  kb.ArrowRight,
		"ArrowUp":     kb.ArrowUp,
		"ArrowDown":   kb.ArrowDown,
		"Home":        kb.Home,
		"End":         kb.End,
		"PageUp":      kb.PageUp,
		"PageDown":    kb.PageDown,
		"Insert":      kb.Insert,
		"F1":          kb.F1,
		"F2":          kb.F2,
		"F3":          kb.F3,
		"F4":          kb.F4,
		"F5":          kb.F5,
		"F6":          kb.F6,
		"F7":          kb.F7,
		"F8":          kb.F8,
		"F9":          kb.F9,
		"F10":         kb.F10,
		"F11":         kb.F11,
		"F12":         kb.F12,
	}

	resolvedKey := key
	if mapped, ok := keyNames[key]; ok {
		resolvedKey = mapped
	}

	var actions []chromedp.Action
	if *sel != "" {
		actions = append(actions,
			chromedp.WaitVisible(*sel, chromedp.ByQuery),
			chromedp.Focus(*sel, chromedp.ByQuery),
		)
	}
	actions = append(actions, chromedp.KeyEvent(resolvedKey))

	if err := chromedp.Run(tabCtx, actions...); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if *sel != "" {
		fmt.Printf("%s✓%s key %q on %s\n", ansiGreen, ansiReset, key, *sel)
	} else {
		fmt.Printf("%s✓%s key %q\n", ansiGreen, ansiReset, key)
	}
}
