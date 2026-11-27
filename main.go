package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"browser-tools/browser"
	"browser-tools/cmd"

	flag "github.com/spf13/pflag"
)

var (
	browserVariant = flag.String("browser", browserDefault(), "browser variant: chrome-stable, chrome-beta, chrome-dev, chrome-canary, cft-stable, cft-beta, cft-dev, cft-canary")
	port           = flag.Int("port", 9222, "remote debugging port")
	timeout        = flag.Duration("timeout", 10*time.Second, "how long commands wait for elements and page loads")
	dialog         = flag.String("dialog", "accept", "answer to alert/confirm/prompt dialogs: accept, dismiss, or the text for a prompt")
)

// globalFlags may be written after the command name as well, so
// `wait #x --timeout 30s` works the same as `--timeout 30s wait #x`.
var globalFlags = map[string]bool{"browser": true, "port": true, "timeout": true, "dialog": true}

// longRunning lists the commands that stream until the user stops them, so the
// watchdog stays disarmed for them.
var longRunning = map[string]bool{
	"network":      true,
	"intercept":    true,
	"pick-element": true,
	"start":        true,
	"update":       true,
	"update-cft":   true,

	browser.HolderCommand: true,
	browser.KeeperCommand: true,
}

// armWatchdog ends the process once the command budget is spent. The timeout is
// enforced here instead of through a context deadline: chromedp closes the tab a
// cancelled context is attached to, which would take the user's tab with it.
// Commands stop a little earlier and report a precise error themselves, so this
// only fires when the browser itself stopped answering.
func armWatchdog(d time.Duration, command string) {
	time.AfterFunc(d+500*time.Millisecond, func() {
		fmt.Fprintf(os.Stderr, "error: `%s` got no answer from the browser within %s\n", command, d)
		fmt.Fprintf(os.Stderr, "  raise --timeout, or check `browser-tools tab` to see whether the browser is stuck\n")
		os.Exit(1)
	})
}

func browserDefault() string {
	if v := os.Getenv("BROWSER_TOOLS_BROWSER"); v != "" {
		return v
	}
	return "chrome-stable"
}

// hoistGlobalFlags moves global flags that appear after the command name to the
// front, so their position never matters.
func hoistGlobalFlags(argv []string) []string {
	var head, tail []string
	seenCommand := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		name, hasValue := strings.CutPrefix(a, "--")
		if !hasValue {
			if !strings.HasPrefix(a, "-") {
				seenCommand = true
			}
			tail = append(tail, a)
			continue
		}
		key, inline, joined := strings.Cut(name, "=")
		if !seenCommand || !globalFlags[key] {
			tail = append(tail, a)
			continue
		}
		if joined {
			head = append(head, "--"+key+"="+inline)
			continue
		}
		if i+1 < len(argv) {
			head = append(head, "--"+key, argv[i+1])
			i++
		}
	}
	return append(head, tail...)
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: browser-tools [options] <command> [args]\n\n")
	fmt.Fprintf(os.Stderr, "Options (accepted before or after the command):\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nSelectors: a CSS selector, an XPath (starts with /), or text=Some Label\n")
	fmt.Fprintf(os.Stderr, "\nCommands:\n")
	cmds := [][2]string{
		{"check [--uncheck] [--force] <sel>", "Check/uncheck a checkbox or radio button"},
		{"clear [options]", "Clear browser data (cookies, cache, storage, …)"},
		{"console [--errors-only]", "Get console messages from the current tab"},
		{"cookie [--all]", "List cookies for the current tab"},
		{"dom-storage [--local|--session]", "Show localStorage / sessionStorage"},
		{"download [--output <path>] <sel>", "Download a file by clicking an element"},
		{"element [--attr <name>] <sel>", "Read text, value, attributes, box and style as JSON"},
		{"evaluate-js [JS]", "Evaluate JavaScript in the current tab"},
		{"extension <command> [args]", "Load/list/uninstall/trigger extensions (Chrome for Testing)"},
		{"fill [--clear] <sel> <text>", "Fill an input field"},
		{"html [options]", "Get the page HTML, optionally filtered"},
		{"intercept <action> [options]", "Block/redirect/modify/mock network requests"},
		{"key [--selector <sel>] <key>", "Simulate a key press"},
		{"mouse <action> [options] <sel>", "Simulate mouse actions (click/dblclick/hover/…)"},
		{"navigate [--new-tab] <url>", "Open a URL and wait for it to load"},
		{"network [options]", "Capture network requests"},
		{"pdf [options]", "Save the current page as a PDF"},
		{"pick-element <message>", "Interactively pick a DOM element"},
		{"reload [--hard]", "Reload the current tab"},
		{"resize [options] <w> <h>", "Emulate a viewport size or phone, held until --reset"},
		{"screenshot [--selector <sel>]", "Take a screenshot"},
		{"scroll [<sel>|--x|--y|--top|--bottom]", "Scroll to an element or X/Y position"},
		{"select [options] <sel> <val>", "Select a dropdown option"},
		{"start", "Start the browser with remote debugging"},
		{"tab [options]", "Manage tabs"},
		{"update [--check] [--force]", "Update browser-tools to the latest release"},
		{"update-cft [--channel <c>]", "Download or update a Chrome for Testing build"},
		{"upload <sel> <file> [file2 …]", "Set files on a file input"},
		{"wait [--hidden|--present|--absent] [sel]", "Wait for an element, or for the page to go idle"},
	}
	for _, c := range cmds {
		fmt.Fprintf(os.Stderr, "  %-38s %s\n", c[0], c[1])
	}
}

func main() {
	flag.Usage = usage
	flag.SetInterspersed(false)
	flag.CommandLine.Parse(hoistGlobalFlags(os.Args[1:]))

	if flag.NArg() < 1 {
		usage()
		os.Exit(1)
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	bgCtx := context.Background()

	command := flag.Arg(0)
	// Streaming commands stay attached for minutes and leave dialogs to the
	// commands that act on the page.
	if !longRunning[command] {
		armWatchdog(*timeout, command)
		browser.DialogAnswer = *dialog
	}

	switch command {
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
	case "element":
		cmd.Element(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "evaluate-js":
		cmd.EvaluateJS(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "extension", "extensions":
		cmd.Extension(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "fill":
		cmd.Fill(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "html":
		cmd.HTML(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "intercept":
		cmd.Intercept(bgCtx, *browserVariant, *port, flag.Args()[1:])
	case "key":
		cmd.Key(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "mouse":
		cmd.Mouse(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "navigate":
		cmd.Navigate(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "network":
		cmd.Network(bgCtx, *browserVariant, *port, flag.Args()[1:])
	case "pdf":
		cmd.PDF(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "pick-element":
		cmd.PickElement(bgCtx, *browserVariant, *port, flag.Args()[1:])
	case "reload":
		cmd.Reload(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "resize":
		cmd.Resize(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case browser.HolderCommand:
		cmd.HoldEmulation(bgCtx, *browserVariant, *port, flag.Args()[1:])
	case browser.KeeperCommand:
		if err := browser.RunDialogKeeper(*port); err != nil {
			os.Exit(1)
		}
	case "screenshot":
		cmd.Screenshot(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "scroll":
		cmd.Scroll(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "select":
		cmd.SelectDropdown(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "upload":
		cmd.Upload(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "wait":
		cmd.Wait(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "start":
		cmd.Start(*browserVariant, *port, flag.Args()[1:])
	case "tab":
		cmd.Tab(timeoutCtx, *browserVariant, *port, flag.Args()[1:])
	case "update":
		cmd.Update(bgCtx, flag.Args()[1:])
	case "update-cft":
		cmd.CfT(bgCtx, flag.Args()[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", command)
		usage()
		os.Exit(1)
	}
}
