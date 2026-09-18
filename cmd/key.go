package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	flag "github.com/spf13/pflag"
)

var keyNames = map[string]string{
	"Enter": kb.Enter, "Tab": kb.Tab, "Backspace": kb.Backspace, "Escape": kb.Escape,
	"Delete": kb.Delete, "ArrowLeft": kb.ArrowLeft, "ArrowRight": kb.ArrowRight,
	"ArrowUp": kb.ArrowUp, "ArrowDown": kb.ArrowDown, "Home": kb.Home, "End": kb.End,
	"PageUp": kb.PageUp, "PageDown": kb.PageDown, "Insert": kb.Insert,
	"F1": kb.F1, "F2": kb.F2, "F3": kb.F3, "F4": kb.F4, "F5": kb.F5, "F6": kb.F6,
	"F7": kb.F7, "F8": kb.F8, "F9": kb.F9, "F10": kb.F10, "F11": kb.F11, "F12": kb.F12,
}

func Key(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("key", flag.ExitOnError)
	sel := fs.String("selector", "", "focus this element before pressing the key")
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
	resolvedKey := key
	if mapped, ok := keyNames[key]; ok {
		resolvedKey = mapped
	}

	s := MustSession(ctx, variant, port)

	tabCtx := MustTab(s)
	if *sel != "" {
		m := Locate(ctx, s, *sel, StateVisible)
		tabCtx = m.Tab
		if err := Focus(tabCtx); err != nil {
			fail("%v", err)
		}
	}

	settle := SettleAfter(tabCtx)
	if err := RunBounded(tabCtx, chromedp.KeyEvent(resolvedKey)); err != nil {
		fail("%v", err)
	}

	settle()
	if *sel != "" {
		fmt.Printf("%s✓%s key %q on %s\n", ansiGreen, ansiReset, key, *sel)
	} else {
		fmt.Printf("%s✓%s key %q\n", ansiGreen, ansiReset, key)
	}
}
