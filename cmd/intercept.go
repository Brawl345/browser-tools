package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"browser-tools/browser"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

// actionFor builds the Fetch action and the log line for one paused request.
type actionFor func(e *fetch.EventRequestPaused, n int) (chromedp.Action, string)

func Intercept(ctx context.Context, variant string, port int, args []string) {
	if len(args) < 1 {
		interceptUsage()
		os.Exit(1)
	}

	sub, rest := args[0], args[1:]
	switch sub {
	case "block":
		interceptBlock(variant, port, rest)
	case "redirect":
		interceptRedirect(variant, port, rest)
	case "modify":
		interceptModify(variant, port, rest)
	case "mock":
		interceptMock(variant, port, rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown intercept action: %s\n\n", sub)
		interceptUsage()
		os.Exit(1)
	}
}

func interceptUsage() {
	fmt.Fprintln(os.Stderr, "Usage: browser-tools intercept <action> [options]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Actions:")
	fmt.Fprintln(os.Stderr, "  block      Fail matching requests")
	fmt.Fprintln(os.Stderr, "  redirect   Reroute matching requests to another URL")
	fmt.Fprintln(os.Stderr, "  modify     Add/override/remove request headers")
	fmt.Fprintln(os.Stderr, "  mock       Answer matching requests with a custom response")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Run 'intercept <action> --help' for action-specific options.")
}

// selectFlags registers the request-selection flags shared by every action.
func selectFlags(fs *flag.FlagSet) (urlPattern, resType *string) {
	urlPattern = fs.String("url", "*", "wildcard URL pattern to intercept (* = any, ? = one char)")
	resType = fs.String("type", "all", "filter by resource type: all, xhr, fetch, document, script, stylesheet, image, font, media, websocket, other")
	return
}

func newActionFlagSet(name, summary string) *flag.FlagSet {
	fs := flag.NewFlagSet("intercept "+name, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: browser-tools intercept %s [options]\n\n%s\n\n", name, summary)
		fs.PrintDefaults()
	}
	return fs
}

func interceptBlock(variant string, port int, args []string) {
	fs := newActionFlagSet("block", "Fail matching requests (ERR_BLOCKED_BY_CLIENT).")
	urlPattern, resType := selectFlags(fs)
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	runIntercept(variant, port, *urlPattern, *resType, func(e *fetch.EventRequestPaused, n int) (chromedp.Action, string) {
		return fetch.FailRequest(e.RequestID, network.ErrorReasonBlockedByClient),
			fmt.Sprintf("%s%s✗ BLOCK #%-4d%s", ansiBold, ansiRed, n, ansiReset)
	})
}

func interceptRedirect(variant string, port int, args []string) {
	fs := newActionFlagSet("redirect <url>", "Reroute matching requests to <url> (transparent to the page).")
	urlPattern, resType := selectFlags(fs)
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "error: redirect requires a target URL")
		fs.Usage()
		os.Exit(1)
	}
	target := fs.Arg(0)
	runIntercept(variant, port, *urlPattern, *resType, func(e *fetch.EventRequestPaused, n int) (chromedp.Action, string) {
		return fetch.ContinueRequest(e.RequestID).WithURL(target),
			fmt.Sprintf("%s%s↪ REDIR #%-4d%s → %s", ansiBold, ansiYellow, n, ansiReset, target)
	})
}

func interceptModify(variant string, port int, args []string) {
	fs := newActionFlagSet("modify", "Add/override or remove request headers, then continue.")
	urlPattern, resType := selectFlags(fs)
	setHeaders := fs.StringArray("set-header", nil, "add/override a request header, format \"Name: Value\" (repeatable)")
	removeHeaders := fs.StringArray("remove-header", nil, "remove a request header by name (repeatable)")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if len(*setHeaders) == 0 && len(*removeHeaders) == 0 {
		fmt.Fprintln(os.Stderr, "error: modify requires at least one --set-header or --remove-header")
		fs.Usage()
		os.Exit(1)
	}

	setEntries, err := parseHeaderArgs(*setHeaders)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	removeSet := make(map[string]struct{}, len(*removeHeaders))
	for _, name := range *removeHeaders {
		removeSet[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}

	runIntercept(variant, port, *urlPattern, *resType, func(e *fetch.EventRequestPaused, n int) (chromedp.Action, string) {
		hdrs := mergeHeaders(e.Request.Headers, setEntries, removeSet)
		return fetch.ContinueRequest(e.RequestID).WithHeaders(hdrs),
			fmt.Sprintf("%s%s✎ MODIFY #%-4d%s headers", ansiBold, ansiYellow, n, ansiReset)
	})
}

func interceptMock(variant string, port int, args []string) {
	fs := newActionFlagSet("mock", "Answer matching requests with a custom response (the server is not hit).")
	urlPattern, resType := selectFlags(fs)
	status := fs.Int("status", 200, "response status code")
	body := fs.String("body", "", "response body")
	file := fs.String("file", "", "response body read from a file")
	contentType := fs.String("content-type", "", "content-type response header")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	mockBody := *body
	if *file != "" {
		data, err := os.ReadFile(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading --file: %v\n", err)
			os.Exit(1)
		}
		mockBody = string(data)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(mockBody))

	runIntercept(variant, port, *urlPattern, *resType, func(e *fetch.EventRequestPaused, n int) (chromedp.Action, string) {
		p := fetch.FulfillRequest(e.RequestID, int64(*status)).WithBody(encoded)
		if *contentType != "" {
			p = p.WithResponseHeaders([]*fetch.HeaderEntry{{Name: "Content-Type", Value: *contentType}})
		}
		return p, fmt.Sprintf("%s%s★ MOCK  #%-4d%s %d", ansiBold, ansiGreen, n, ansiReset, *status)
	})
}

// runIntercept drives the shared interception loop: it pauses every request matching the
// pattern, logs it, and applies the action produced by build. Blocks until SIGINT.
func runIntercept(variant string, port int, urlPattern, resTypeFlag string, build actionFor) {
	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(context.Background(), port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	pattern := &fetch.RequestPattern{URLPattern: urlPattern}
	if tf := strings.ToLower(resTypeFlag); tf != "all" {
		pattern.ResourceType = resourceTypeFor(tf)
	}

	var (
		mu    sync.Mutex
		count int
	)

	chromedp.ListenTarget(tabCtx, func(ev any) {
		e, ok := ev.(*fetch.EventRequestPaused)
		if !ok {
			return
		}

		mu.Lock()
		count++
		n := count
		mu.Unlock()

		rt := strings.ToLower(string(e.ResourceType))
		fmt.Printf("\n%s%s⏸ INT   #%-4d%s %s%-7s%s %s[%-11s]%s %s\n",
			ansiBold, ansiCyan, n, ansiReset,
			ansiBold, e.Request.Method, ansiReset,
			ansiDim, rt, ansiReset,
			e.Request.URL)

		action, line := build(e, n)
		go func() {
			if err := chromedp.Run(tabCtx, action); err != nil {
				fmt.Printf("  %s[#%-4d error: %v]%s\n", ansiDim, n, err, ansiReset)
				return
			}
			fmt.Println(line)
		}()
	})

	enable := fetch.Enable().WithPatterns([]*fetch.RequestPattern{pattern})
	if err := chromedp.Run(tabCtx, enable); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "%sIntercepting requests matching %q (Ctrl+C to stop)...%s\n", ansiDim, urlPattern, ansiReset)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	mu.Lock()
	total := count
	mu.Unlock()
	fmt.Fprintf(os.Stderr, "\n%sStopped. Total requests intercepted: %d%s\n", ansiBold, total, ansiReset)
}

// parseHeaderArgs parses "Name: Value" strings into fetch header entries.
func parseHeaderArgs(args []string) ([]*fetch.HeaderEntry, error) {
	var entries []*fetch.HeaderEntry
	for _, a := range args {
		name, value, ok := strings.Cut(a, ":")
		if !ok {
			return nil, fmt.Errorf("invalid --set-header %q (expected \"Name: Value\")", a)
		}
		entries = append(entries, &fetch.HeaderEntry{
			Name:  strings.TrimSpace(name),
			Value: strings.TrimSpace(value),
		})
	}
	return entries, nil
}

// mergeHeaders applies set/remove edits to the original request headers.
func mergeHeaders(orig network.Headers, set []*fetch.HeaderEntry, remove map[string]struct{}) []*fetch.HeaderEntry {
	overrides := make(map[string]struct{}, len(set))
	for _, h := range set {
		overrides[strings.ToLower(h.Name)] = struct{}{}
	}
	var out []*fetch.HeaderEntry
	for k, v := range orig {
		lk := strings.ToLower(k)
		if _, drop := remove[lk]; drop {
			continue
		}
		if _, replaced := overrides[lk]; replaced {
			continue
		}
		out = append(out, &fetch.HeaderEntry{Name: k, Value: fmt.Sprintf("%v", v)})
	}
	return append(out, set...)
}

// resourceTypeFor maps a lowercase filter string to a network.ResourceType.
func resourceTypeFor(t string) network.ResourceType {
	switch t {
	case "document":
		return network.ResourceTypeDocument
	case "stylesheet":
		return network.ResourceTypeStylesheet
	case "image":
		return network.ResourceTypeImage
	case "media":
		return network.ResourceTypeMedia
	case "font":
		return network.ResourceTypeFont
	case "script":
		return network.ResourceTypeScript
	case "xhr":
		return network.ResourceTypeXHR
	case "fetch":
		return network.ResourceTypeFetch
	case "websocket":
		return network.ResourceTypeWebSocket
	case "other":
		return network.ResourceTypeOther
	default:
		return network.ResourceType(t)
	}
}
