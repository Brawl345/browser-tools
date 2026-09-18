package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Upload(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools upload <selector> <file> [file2 ...]")
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

	sel := fs.Arg(0)
	rawPaths := fs.Args()[1:]

	// Resolve all paths to absolute and verify they exist.
	files := make([]string, 0, len(rawPaths))
	for _, p := range rawPaths {
		abs, err := filepath.Abs(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error resolving path %q: %v\n", p, err)
			os.Exit(1)
		}
		if _, err := os.Stat(abs); err != nil {
			fmt.Fprintf(os.Stderr, "error: file not found: %s\n", abs)
			os.Exit(1)
		}
		files = append(files, abs)
	}

	s := MustSession(ctx, variant, port)
	m := Locate(ctx, s, sel, StatePresent)
	if m.Path == "" {
		fail("%s sits inside a shadow root — file inputs must be in the main document", sel)
	}

	if err := RunBounded(m.Tab, chromedp.SetUploadFiles(m.Path, files, chromedp.ByQuery)); err != nil {
		fail("%v", err)
	}

	for _, f := range files {
		fmt.Printf("%s✓%s %s\n", ansiGreen, ansiReset, f)
	}
}
