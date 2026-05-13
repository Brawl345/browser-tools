package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"browser-tools/browser"
	flag "github.com/spf13/pflag"
	"github.com/chromedp/chromedp"
)

func HTML(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("html", flag.ExitOnError)
	filter := fs.String("filter", "", "regex filter")
	lines := fs.Int("lines", 5, "context lines around each match")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools html [--filter <regex>] [--lines <n>]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	var html string
	if err := chromedp.Run(tabCtx, chromedp.OuterHTML("html", &html, chromedp.ByQuery)); err != nil {
		fmt.Fprintf(os.Stderr, "error getting HTML: %v\n", err)
		os.Exit(1)
	}

	if *filter == "" {
		fmt.Println(html)
		return
	}

	out, err := filterWithContext(html, *filter, *lines)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if out == "" {
		fmt.Fprintln(os.Stderr, "no matches found")
		os.Exit(1)
	}
	fmt.Print(out)
}

func filterWithContext(text, pattern string, contextLines int) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex: %w", err)
	}

	lines := strings.Split(text, "\n")

	var ranges [][2]int
	for i, line := range lines {
		if re.MatchString(line) {
			ranges = append(ranges, [2]int{
				max(0, i-contextLines),
				min(len(lines)-1, i+contextLines),
			})
		}
	}

	if len(ranges) == 0 {
		return "", nil
	}

	merged := [][2]int{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r[0] <= last[1]+1 {
			last[1] = max(last[1], r[1])
		} else {
			merged = append(merged, r)
		}
	}

	var sb strings.Builder
	for i, r := range merged {
		if i > 0 {
			sb.WriteString("--\n")
		}
		for _, line := range lines[r[0] : r[1]+1] {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	return sb.String(), nil
}
