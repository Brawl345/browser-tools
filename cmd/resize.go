package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"browser-tools/browser"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Resize(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("resize", flag.ExitOnError)
	reset := fs.Bool("reset", false, "clear the viewport override and restore the default size")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools resize <width> <height>")
		fmt.Fprintln(os.Stderr, "       browser-tools resize --reset")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	var action chromedp.Action
	var result string
	if *reset {
		// The override set by a previous resize lives on after that process's
		// CDP session detached. A bare ClearDeviceMetricsOverride from a fresh
		// session is a no-op because this session never enabled emulation, so
		// first re-assert an override in this session, then clear it.
		action = chromedp.Tasks{
			emulation.SetDeviceMetricsOverride(1, 1, 1, false),
			emulation.ClearDeviceMetricsOverride(),
		}
		result = "reset viewport to default"
	} else {
		if fs.NArg() < 2 {
			fs.Usage()
			os.Exit(1)
		}
		width, err := strconv.ParseInt(fs.Arg(0), 10, 64)
		if err != nil || width <= 0 {
			fmt.Fprintf(os.Stderr, "error: invalid width %q\n", fs.Arg(0))
			os.Exit(1)
		}
		height, err := strconv.ParseInt(fs.Arg(1), 10, 64)
		if err != nil || height <= 0 {
			fmt.Fprintf(os.Stderr, "error: invalid height %q\n", fs.Arg(1))
			os.Exit(1)
		}
		action = chromedp.EmulateViewport(width, height)
		result = fmt.Sprintf("resized viewport to %dx%d", width, height)
	}

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	if err := chromedp.Run(tabCtx, action); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s✓%s %s\n", ansiGreen, ansiReset, result)
}
