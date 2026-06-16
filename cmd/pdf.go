package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"browser-tools/browser"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

// paperFormats maps a format name to its width and height in inches.
var paperFormats = map[string][2]float64{
	"a3":      {11.69, 16.54},
	"a4":      {8.27, 11.69},
	"a5":      {5.83, 8.27},
	"letter":  {8.5, 11},
	"legal":   {8.5, 14},
	"tabloid": {11, 17},
}

const (
	defaultHeaderTemplate = `<div style="font-size:9px; width:100%; text-align:center; color:#666;"><span class="title"></span></div>`
	defaultFooterTemplate = `<div style="font-size:9px; width:100%; text-align:center; color:#666;"><span class="pageNumber"></span> / <span class="totalPages"></span></div>`
)

func PDF(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("pdf", flag.ExitOnError)
	output := fs.String("output", "", "output path (default: /tmp/page-YYYYMMDD-HHMMSS.pdf)")
	format := fs.String("format", "a4", "paper format: a3, a4, a5, letter, legal, tabloid")
	landscape := fs.Bool("landscape", false, "use landscape orientation")
	margin := fs.Float64("margin", 0.4, "page margin in inches (applied to all sides)")
	scale := fs.Float64("scale", 1.0, "scale of the page rendering (0.1–2)")
	noBackground := fs.Bool("no-background", false, "do not print background graphics")
	headerFooter := fs.Bool("header-footer", false, "show a header and footer on each page")
	headerTemplate := fs.String("header-template", "", "custom HTML header template (implies --header-footer)")
	footerTemplate := fs.String("footer-template", "", "custom HTML footer template (implies --header-footer)")
	pageRanges := fs.String("page-ranges", "", "pages to print, one-based, e.g. '1-5, 8, 11-13'")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools pdf [options]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	paper, ok := paperFormats[strings.ToLower(*format)]
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unknown format %q (use a3, a4, a5, letter, legal or tabloid)\n", *format)
		os.Exit(1)
	}

	header := *headerTemplate
	footer := *footerTemplate
	showHeaderFooter := *headerFooter || header != "" || footer != ""
	if showHeaderFooter {
		if header == "" {
			header = defaultHeaderTemplate
		}
		if footer == "" {
			footer = defaultFooterTemplate
		}
	}

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	params := page.PrintToPDF().
		WithPaperWidth(paper[0]).
		WithPaperHeight(paper[1]).
		WithLandscape(*landscape).
		WithScale(*scale).
		WithMarginTop(*margin).
		WithMarginBottom(*margin).
		WithMarginLeft(*margin).
		WithMarginRight(*margin).
		WithPrintBackground(!*noBackground).
		WithDisplayHeaderFooter(showHeaderFooter).
		WithHeaderTemplate(header).
		WithFooterTemplate(footer).
		WithPageRanges(*pageRanges)

	var buf []byte
	if err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		buf, _, err = params.Do(ctx)
		return err
	})); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var path string
	if *output != "" {
		path = *output
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating output directory: %v\n", err)
			os.Exit(1)
		}
	} else {
		path = fmt.Sprintf("/tmp/page-%s.pdf", time.Now().Format("20060102-150405"))
	}

	if err := os.WriteFile(path, buf, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error saving PDF: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(path)
}
