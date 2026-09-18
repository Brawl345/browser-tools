package cmd

import (
	"context"
	"fmt"
	"os"

	flag "github.com/spf13/pflag"
)

func Wait(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("wait", flag.ExitOnError)
	visible := fs.Bool("visible", false, "wait until the element is visible (default)")
	hidden := fs.Bool("hidden", false, "wait until the element is present but not visible")
	present := fs.Bool("present", false, "wait until the element exists in the DOM")
	absent := fs.Bool("absent", false, "wait until the element is gone from the DOM")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools wait [--visible | --hidden | --present | --absent] [selector]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Waits for an element to reach a state, then exits. Without a selector it")
		fmt.Fprintln(os.Stderr, "waits until the page has loaded and network traffic has stopped.")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	modes := 0
	for _, m := range []bool{*visible, *hidden, *present, *absent} {
		if m {
			modes++
		}
	}
	if modes > 1 {
		fail("choose only one of --visible, --hidden, --present, --absent")
	}

	var state string
	switch {
	case *hidden:
		state = StateHidden
	case *present:
		state = StatePresent
	case *absent:
		state = StateAbsent
	default:
		state = StateVisible
	}

	s := MustSession(ctx, variant, port)

	if fs.NArg() == 0 {
		tabCtx := MustTab(s)
		w := watch(tabCtx)
		WaitLoaded(ctx, tabCtx, w)
		fmt.Printf("%s✓%s page is idle\n", ansiGreen, ansiReset)
		return
	}

	sel := fs.Arg(0)
	m := Locate(ctx, s, sel, state)
	fmt.Printf("%s✓%s %s is %s%s\n", ansiGreen, ansiReset, sel, state, matchNote(m))
}
