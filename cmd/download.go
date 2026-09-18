package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Download(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("download", flag.ExitOnError)
	output := fs.String("output", "", "output path (default: ~/Downloads/<suggested filename>)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools download [--output <path>] <selector>")
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

	sel := fs.Arg(0)

	s := MustSession(ctx, variant, port)
	m := Locate(ctx, s, sel, StateVisible)
	tabCtx := m.Tab

	// Use a temp dir; files are named by GUID (allowAndName behavior).
	tmpDir, err := os.MkdirTemp("", "browser-tools-dl-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	// Configure download behavior on the Browser level.
	if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		return cdpbrowser.
			SetDownloadBehavior(cdpbrowser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath(tmpDir).
			WithEventsEnabled(true).
			Do(cdp.WithExecutor(ctx, c.Browser))
	})); err != nil {
		fmt.Fprintf(os.Stderr, "error setting download behavior: %v\n", err)
		os.Exit(1)
	}

	type downloadInfo struct {
		guid     string
		filename string
	}

	beginCh := make(chan downloadInfo, 1)
	doneCh := make(chan error, 1)

	chromedp.ListenBrowser(tabCtx, func(ev any) {
		switch e := ev.(type) {
		case *cdpbrowser.EventDownloadWillBegin:
			select {
			case beginCh <- downloadInfo{guid: e.GUID, filename: e.SuggestedFilename}:
			default:
			}
		case *cdpbrowser.EventDownloadProgress:
			switch e.State {
			case cdpbrowser.DownloadProgressStateCompleted:
				select {
				case doneCh <- nil:
				default:
				}
			case cdpbrowser.DownloadProgressStateCanceled:
				select {
				case doneCh <- fmt.Errorf("download was canceled"):
				default:
				}
			}
		}
	})

	// Click the element to trigger the download.
	if err := PointerAt(tabCtx, m.X, m.Y, input.Left, 1); err != nil {
		fail("%v", err)
	}

	// Wait for download to begin.
	var info downloadInfo
	select {
	case info = <-beginCh:
	case <-ctx.Done():
		fmt.Fprintln(os.Stderr, "error: timed out waiting for download to start")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "%sdownloading:%s %s\n", ansiDim, ansiReset, info.filename)

	// Wait for download to complete (use a longer context — the file might be large).
	select {
	case err := <-doneCh:
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		fmt.Fprintln(os.Stderr, "error: timed out waiting for download to complete")
		os.Exit(1)
	}

	// Determine destination path.
	src := filepath.Join(tmpDir, info.guid)
	var dst string
	if *output != "" {
		dst = *output
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating output directory: %v\n", err)
			os.Exit(1)
		}
	} else {
		home, _ := os.UserHomeDir()
		dst = filepath.Join(home, "Downloads", info.filename)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating downloads directory: %v\n", err)
			os.Exit(1)
		}
	}

	if err := os.Rename(src, dst); err != nil {
		// Rename across devices fails — fall back to copy.
		if err2 := copyFile(src, dst); err2 != nil {
			fmt.Fprintf(os.Stderr, "error saving file: %v\n", err2)
			os.Exit(1)
		}
	}

	fmt.Println(dst)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
