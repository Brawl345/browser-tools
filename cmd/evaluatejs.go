package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"browser-tools/browser"
	flag "github.com/spf13/pflag"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func EvaluateJS(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("evaluate-js", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools evaluate-js [JAVASCRIPT]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "JAVASCRIPT can be either inline code, a path to a .js file, or '-' to read from stdin.")
		fmt.Fprintln(os.Stderr, "If no argument is provided, reads from stdin.")
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	jsCode, err := loadJS(fs.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	var result []byte
	awaitPromise := func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}
	if err := chromedp.Run(tabCtx, chromedp.Evaluate(jsCode, &result, awaitPromise)); err != nil {
		switch err {
		case chromedp.ErrJSUndefined, chromedp.ErrJSNull:
		default:
			fmt.Fprintf(os.Stderr, "evaluate error: %v\n", err)
			os.Exit(1)
		}
	}

	if len(result) > 0 {
		fmt.Println(string(result))
	}
}

func loadJS(args []string) (string, error) {
	if len(args) == 0 || args[0] == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(data), nil
	}
	arg := args[0]
	if info, err := os.Stat(arg); err == nil && !info.IsDir() {
		data, err := os.ReadFile(arg)
		if err != nil {
			return "", fmt.Errorf("reading file %s: %w", arg, err)
		}
		return string(data), nil
	}
	return arg, nil
}
