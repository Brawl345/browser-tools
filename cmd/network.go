package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"browser-tools/browser"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiCyan   = "\033[36m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
	ansiBlue   = "\033[34m"
)

type capturedRequest struct {
	num     int
	id      network.RequestID
	url     string
	method  string
	resType network.ResourceType
	headers network.Headers
	body    string
}

func Network(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("network", flag.ExitOnError)
	resTypeFlag := fs.String("type", "all", "filter by resource type: all, xhr, fetch, document, script, stylesheet, image, font, media, websocket, other")
	showHeaders := fs.Bool("show-headers", false, "show request and response headers")
	showBody := fs.Bool("show-body", false, "show response body (xhr/fetch only)")
	urlFilter := fs.String("filter", "", "filter URLs by regex")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools network [options]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	var urlRe *regexp.Regexp
	if *urlFilter != "" {
		var err error
		urlRe, err = regexp.Compile(*urlFilter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --filter regex: %v\n", err)
			os.Exit(1)
		}
	}

	typeFilter := strings.ToLower(*resTypeFlag)

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(context.Background(), port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	var (
		mu       sync.Mutex
		requests = make(map[network.RequestID]*capturedRequest)
		count    int
	)

	matches := func(url string, rt network.ResourceType) bool {
		if typeFilter != "all" && !strings.EqualFold(string(rt), typeFilter) {
			return false
		}
		if urlRe != nil && !urlRe.MatchString(url) {
			return false
		}
		return true
	}

	statusColor := func(status int64) string {
		switch {
		case status >= 500:
			return ansiRed
		case status >= 400:
			return ansiRed
		case status >= 300:
			return ansiYellow
		default:
			return ansiGreen
		}
	}

	printHeaders := func(label, color string, headers network.Headers) {
		fmt.Printf("  %s%s%s\n", color, label, ansiReset)
		for k, v := range headers {
			fmt.Printf("    %s%s%s: %v\n", ansiDim, k, ansiReset, v)
		}
	}

	chromedp.ListenTarget(tabCtx, func(ev any) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if e.RedirectResponse != nil {
				return
			}
			if !matches(e.Request.URL, e.Type) {
				return
			}

			mu.Lock()
			count++
			n := count
			req := &capturedRequest{
				num:     n,
				id:      e.RequestID,
				url:     e.Request.URL,
				method:  e.Request.Method,
				resType: e.Type,
				headers: e.Request.Headers,
				body:    "",
			}
			if e.Request.HasPostData {
				var parts []string
				for _, entry := range e.Request.PostDataEntries {
					if entry.Bytes != "" {
						parts = append(parts, entry.Bytes)
					}
				}
				req.body = strings.Join(parts, "")
			}
			requests[e.RequestID] = req
			mu.Unlock()

			rt := strings.ToLower(string(e.Type))
			fmt.Printf("\n%s%s→ REQ  #%-4d%s %s%-7s%s %s[%-11s]%s %s\n",
				ansiBold, ansiCyan, n, ansiReset,
				ansiBold, e.Request.Method, ansiReset,
				ansiDim, rt, ansiReset,
				e.Request.URL)

			if *showHeaders {
				printHeaders("Request Headers:", ansiBlue, e.Request.Headers)
			}
			if *showBody && req.body != "" {
				fmt.Printf("  %sRequest Body:%s\n    %s\n", ansiBlue, ansiReset, req.body)
			}

		case *network.EventResponseReceived:
			mu.Lock()
			req, ok := requests[e.RequestID]
			mu.Unlock()
			if !ok {
				return
			}

			sc := statusColor(e.Response.Status)
			fmt.Printf("%s%s← RES  #%-4d%s %s%d %s%s %s\n",
				ansiBold, sc, req.num, ansiReset,
				sc, e.Response.Status, e.Response.StatusText, ansiReset,
				e.Response.URL)

			if *showHeaders {
				printHeaders("Response Headers:", ansiGreen, e.Response.Headers)
			}

		case *network.EventLoadingFinished:
			if !*showBody {
				return
			}
			mu.Lock()
			req, ok := requests[e.RequestID]
			mu.Unlock()
			if !ok {
				return
			}
			rt := strings.ToLower(string(req.resType))
			if rt != "xhr" && rt != "fetch" {
				return
			}

			reqID := e.RequestID
			num := req.num
			go func() {
				var body []byte
				if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
					var err error
					body, err = network.GetResponseBody(reqID).Do(ctx)
					return err
				})); err != nil {
					fmt.Printf("  %s[BODY  #%-4d]%s <error: %v>\n", ansiDim, num, ansiReset, err)
					return
				}
				bodyStr := strings.TrimSpace(string(body))
				if bodyStr == "" {
					return
				}
				fmt.Printf("  %s[BODY  #%-4d]%s %s\n", ansiDim, num, ansiReset, bodyStr)
			}()

		case *network.EventLoadingFailed:
			mu.Lock()
			req, ok := requests[e.RequestID]
			mu.Unlock()
			if !ok {
				return
			}
			fmt.Printf("%s%s✗ FAIL #%-4d%s %s — %s\n",
				ansiBold, ansiRed, req.num, ansiReset,
				req.url, e.ErrorText)
		}
	})

	if err := chromedp.Run(tabCtx, network.Enable()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "%sCatching network requests (Ctrl+C to stop)...%s\n", ansiDim, ansiReset)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	mu.Lock()
	total := count
	mu.Unlock()
	fmt.Fprintf(os.Stderr, "\n%sStopped. Total requests captured: %d%s\n", ansiBold, total, ansiReset)
}
