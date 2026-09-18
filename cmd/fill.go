package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/chromedp/cdproto/input"
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

	s := MustSession(ctx, variant, port)
	m := Locate(ctx, s, selector, StateVisible)

	settle := SettleAfter(m.Tab)
	if err := PointerAt(m.Tab, m.X, m.Y, input.Left, 1); err != nil {
		fail("%v", err)
	}
	if err := Focus(m.Tab); err != nil {
		fail("%v", err)
	}
	if *clear {
		if err := OnHit(m.Tab, `if ("value" in el) {
				el.value = "";
			} else if (el.isContentEditable) {
				el.textContent = "";
			}
			el.dispatchEvent(new Event('input',  {bubbles: true}));
			el.dispatchEvent(new Event('change', {bubbles: true}));
			return true;`, nil); err != nil {
			fail("%v", err)
		}
	}
	if err := RunBounded(m.Tab, chromedp.KeyEvent(text)); err != nil {
		fail("%v", err)
	}

	settle()
	fmt.Printf("%s✓%s filled %s%s\n", ansiGreen, ansiReset, selector, matchNote(m))
}
