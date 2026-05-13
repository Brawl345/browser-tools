package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"browser-tools/cmd"
	flag "github.com/spf13/pflag"
)

var (
	browserVariant = flag.String("browser", browserDefault(), "browser variant: chrome-stable, chrome-beta, chrome-dev, chrome-canary")
	port           = flag.Int("port", 9222, "remote debugging port")
	timeout        = flag.Duration("timeout", 10*time.Second, "timeout for commands that wait on elements")
)

func browserDefault() string {
	if v := os.Getenv("BROWSER_TOOLS_BROWSER"); v != "" {
		return v
	}
	return "chrome-stable"
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: browser-tools [options] <command> [args]\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nCommands:\n")
	cmds := [][2]string{
		{"check [--uncheck] [--force] <sel>",  "Check/uncheck a checkbox or radio button"},
		{"clear [options]",                    "Clear browser data (cookies, cache, storage, …)"},
		{"console [--errors-only]",            "Get console messages from the current tab"},
		{"cookie [--all]",                     "List cookies for the current tab"},
		{"dom-storage [--local|--session]",    "Show localStorage / sessionStorage"},
		{"download [--output <path>] <sel>",   "Download a file by clicking an element"},
		{"evaluate-js [JS]",                   "Evaluate JavaScript in the current tab"},
		{"fill [--clear] <sel> <text>",        "Fill an input field"},
		{"html [options]",                     "Get the page HTML, optionally filtered"},
		{"key [--selector <sel>] <key>",        "Simulate a key press"},
		{"mouse <action> [options] <sel>",     "Simulate mouse actions (click/dblclick/hover/…)"},
		{"navigate [--new-tab] <url>",         "Open a URL in a browser tab"},
		{"network [options]",                  "Capture network requests"},
		{"pick-element <message>",             "Interactively pick a DOM element"},
		{"screenshot [--full-page]",           "Take a screenshot to /tmp"},
		{"select [options] <sel> <val>",       "Select a dropdown option"},
		{"start",                              "Start the browser with remote debugging"},
		{"tab [options]",                      "Manage tabs"},
		{"upload <sel> <file> [file2 …]",      "Set files on a file input"},
	}
	for _, c := range cmds {
		fmt.Fprintf(os.Stderr, "  %-34s %s\n", c[0], c[1])
	}
}

func main() {
	flag.Usage = usage
	flag.SetInterspersed(false)
	flag.Parse()

	if flag.NArg() < 1 {
		usage()
		os.Exit(1)
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	bgCtx := context.Background()

	switch flag.Arg(0) {
	case "check":
		cmd.Check(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "clear":
		cmd.Clear(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "console":
		cmd.Console(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "cookie":
		cmd.Cookie(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "dom-storage":
		cmd.DOMStorage(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "download":
		cmd.Download(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "evaluate-js":
		cmd.EvaluateJS(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "fill":
		cmd.Fill(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "html":
		cmd.HTML(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "key":
		cmd.Key(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "mouse":
		cmd.Mouse(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "navigate":
		cmd.Navigate(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "network":
		cmd.Network(bgCtx, *browserVariant, *port, flag.Args()[1:])
	case "pick-element":
		cmd.PickElement(bgCtx, *browserVariant, *port, flag.Args()[1:])
	case "screenshot":
		cmd.Screenshot(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "select":
		cmd.SelectDropdown(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "upload":
		cmd.Upload(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "start":
		cmd.Start(*browserVariant, *port, flag.Args()[1:])
	case "tab":
		cmd.Tab(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", flag.Arg(0))
		usage()
		os.Exit(1)
	}
}
