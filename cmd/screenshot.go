package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Screenshot(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("screenshot", flag.ExitOnError)
	fullPage := fs.Bool("full-page", false, "capture the entire page, not just the viewport")
	selector := fs.String("selector", "", "capture only the element matching this selector")
	output := fs.String("output", "", "write the PNG here instead of /tmp")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools screenshot [--full-page] [--selector <sel>] [--output <path>]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *fullPage && *selector != "" {
		fail("--full-page and --selector are mutually exclusive")
	}

	s := MustSession(ctx, variant, port)

	var buf []byte
	var tabCtx context.Context
	var action chromedp.Action

	switch {
	case *selector != "":
		m := Locate(ctx, s, *selector, StateVisible)
		tabCtx = m.Tab
		var clip page.Viewport
		if err := OnHit(tabCtx, `const r = el.getBoundingClientRect();
			return { x: r.left + scrollX, y: r.top + scrollY, width: r.width, height: r.height, scale: 1 };`, &clip); err != nil {
			fail("%v", err)
		}
		if clip.Width < 1 || clip.Height < 1 {
			fail("%s has a %gx%g box — nothing to capture", *selector, clip.Width, clip.Height)
		}
		action = chromedp.ActionFunc(func(c context.Context) error {
			var err error
			buf, err = page.CaptureScreenshot().WithClip(&clip).WithCaptureBeyondViewport(true).Do(c)
			return err
		})
	case *fullPage:
		tabCtx = MustTab(s)
		action = chromedp.FullScreenshot(&buf, 100)
	default:
		tabCtx = MustTab(s)
		action = chromedp.CaptureScreenshot(&buf)
	}

	if err := chromedp.Run(tabCtx, action); err != nil {
		fail("%v", err)
	}

	path := *output
	if path == "" {
		path = fmt.Sprintf("/tmp/screenshot-%s.png", time.Now().Format("20060102-150405"))
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fail("cannot create %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		fail("cannot save the screenshot: %v", err)
	}

	fmt.Println(path)
}
