package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

// jsHints point at the command that does the same job without a script. They
// are printed as a note, the script still runs.
var jsHints = []struct {
	pattern *regexp.Regexp
	use     string
}{
	{regexp.MustCompile(`\.click\(\)|new (Mouse|Pointer)Event\(`), `mouse click <selector>`},
	{regexp.MustCompile(`scrollIntoView|window\.scrollTo|scrollTop\s*=`), `scroll <selector> or scroll --y <n>`},
	{regexp.MustCompile(`getComputedStyle|getBoundingClientRect`), `element <selector> (its JSON carries box and style)`},
	{regexp.MustCompile(`\.textContent|\.innerText|\.outerHTML`), `element <selector> --attr text|html`},
	{regexp.MustCompile(`\.value\s*=[^=]`), `fill <selector> <text>`},
	{regexp.MustCompile(`\.checked\s*=[^=]`), `check <selector>`},
	{regexp.MustCompile(`localStorage|sessionStorage`), `dom-storage`},
	{regexp.MustCompile(`document\.cookie`), `cookie`},
	{regexp.MustCompile(`location\.reload`), `reload`},
	{regexp.MustCompile(`location\.(href|assign|replace)\s*=`), `navigate <url>`},
	{regexp.MustCompile(`setTimeout|new Promise\(.*setTimeout`), `wait <selector>`},
	{regexp.MustCompile(`\.selectedIndex\s*=|\.selected\s*=`), `select <selector> <value>`},
	{regexp.MustCompile(`\.focus\(\)`), `key <key> --selector <sel>`},
}

// maxHints keeps the note short even when a script does several things.
const maxHints = 2

func EvaluateJS(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("evaluate-js", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools evaluate-js [JAVASCRIPT]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "JAVASCRIPT is inline code, a path to a .js file, or '-' to read from stdin.")
		fmt.Fprintln(os.Stderr, "A top-level return is a syntax error — wrap statements in (function(){ … })().")
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	jsCode, err := loadJS(fs.Args())
	if err != nil {
		fail("%v", err)
	}

	tabCtx := MustTab(MustSession(ctx, variant, port))

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
	printHints(jsCode)
}

func printHints(js string) {
	var uses []string
	for _, h := range jsHints {
		if h.pattern.MatchString(js) {
			uses = append(uses, h.use)
			if len(uses) == maxHints {
				break
			}
		}
	}
	for _, u := range uses {
		fmt.Fprintf(os.Stderr, "hint: `browser-tools %s` does this without a script\n", u)
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
