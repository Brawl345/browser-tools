package cmd

import (
	"context"
	"fmt"
	"os"

	"browser-tools/browser"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Fill(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("fill", flag.ExitOnError)
	clear := fs.Bool("clear", false, "clear the field before typing")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools fill [--clear] <selector> <text>")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() < 2 {
		fs.Usage()
		os.Exit(1)
	}

	selector, text := fs.Arg(0), fs.Arg(1)

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	actions := []chromedp.Action{
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Click(selector, chromedp.ByQuery),
	}
	if *clear {
		actions = append(actions, chromedp.Evaluate(
			fmt.Sprintf(`(function(){
				var el = document.querySelector(%q);
				if (!el) return;
				el.select();
				el.value = '';
				el.dispatchEvent(new Event('input', {bubbles:true}));
				el.dispatchEvent(new Event('change', {bubbles:true}));
			}())`, selector),
			nil,
		))
	}
	actions = append(actions,
		chromedp.SendKeys(selector, text, chromedp.ByQuery),
	)

	if err := chromedp.Run(tabCtx, actions...); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: use --timeout to increase the wait time\n")
		os.Exit(1)
	}

	fmt.Printf("%s✓%s filled %s\n", ansiGreen, ansiReset, selector)
}
