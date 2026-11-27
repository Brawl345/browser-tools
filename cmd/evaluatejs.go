package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

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
		fmt.Fprintln(os.Stderr, "Top-level await and return work. A string prints as is, anything else as JSON.")
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	jsCode, err := loadJS(fs.Args())
	if err != nil {
		fail("%v", err)
	}

	tabCtx := MustTab(MustSession(ctx, variant, port))

	var out string
	var exc *runtime.ExceptionDetails
	err = chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		out, exc, err = evaluate(ctx, jsCode, true)
		if err == nil && exc != nil && isIllegalReturn(exc) {
			// REPL mode does not await the promise an async wrapper returns.
			out, exc, err = evaluate(ctx, "(async () => {\n"+jsCode+"\n})()", false)
		}
		return err
	}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "evaluate error: %v\n", explainTabError(err))
		os.Exit(1)
	}
	if exc != nil {
		fmt.Fprintf(os.Stderr, "evaluate error: %s\n", exceptionText(exc))
		os.Exit(1)
	}

	if out != "" {
		fmt.Println(out)
	}
	printHints(jsCode)
}

func evaluate(ctx context.Context, js string, repl bool) (string, *runtime.ExceptionDetails, error) {
	res, exc, err := runtime.Evaluate(js).
		WithReplMode(repl).
		WithAwaitPromise(true).
		Do(ctx)
	if err != nil || exc != nil {
		return "", exc, err
	}
	if res.Subtype == runtime.SubtypePromise {
		res, exc, err = runtime.AwaitPromise(res.ObjectID).Do(ctx)
		if err != nil || exc != nil {
			return "", exc, err
		}
	}
	if res.Type != runtime.TypeObject || res.ObjectID == "" || res.Subtype == runtime.SubtypeNode {
		return formatResult(res), nil, nil
	}
	val, vexc, verr := runtime.CallFunctionOn(`function() { return this; }`).
		WithObjectID(res.ObjectID).
		WithReturnByValue(true).
		Do(ctx)
	if verr != nil || vexc != nil || (string(val.Value) == "{}" && res.ClassName != "Object") {
		return res.Description, nil, nil
	}
	return formatResult(val), nil, nil
}

func isIllegalReturn(exc *runtime.ExceptionDetails) bool {
	return strings.Contains(exceptionText(exc), "Illegal return statement")
}

func exceptionText(exc *runtime.ExceptionDetails) string {
	if exc.Exception != nil && exc.Exception.Description != "" {
		return exc.Exception.Description
	}
	if exc.Exception != nil && len(exc.Exception.Value) > 0 {
		return string(exc.Exception.Value)
	}
	return exc.Text
}

func formatResult(res *runtime.RemoteObject) string {
	if res == nil {
		return ""
	}
	switch res.Type {
	case runtime.TypeUndefined:
		return ""
	case runtime.TypeString:
		var s string
		if json.Unmarshal(res.Value, &s) == nil {
			return s
		}
	}
	if len(res.Value) > 0 {
		return string(res.Value)
	}
	if res.UnserializableValue != "" {
		return string(res.UnserializableValue)
	}
	return res.Description
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
	if sleepPattern.MatchString(js) {
		fmt.Fprintln(os.Stderr, "hint: acting commands, `screenshot` and `wait` already wait for animations to end")
	}
}

var sleepPattern = regexp.MustCompile(`setTimeout`)

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
