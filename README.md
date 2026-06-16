# Browser Tools

A coding agent skill for browser automation using Go and the Chrome DevTools Protocol. And yes, this was coded by an AI. Inspired by [this blogpost](https://mariozechner.at/posts/2025-11-02-what-if-you-dont-need-mcp/).

## Features

- **Start Browser**: Launch Chrome with remote debugging, selectable via `--browser` or `BROWSER_TOOLS_BROWSER` (`chrome-stable`, `chrome-beta`, `chrome-dev`, `chrome-canary`)
- **Navigate**: Open URLs in the active or a new tab
- **Execute JavaScript**: Run inline code, files, or STDIN input
- **Element Picker**: Interactive DOM element selection
- **Element Info**: Read a selector's text, attributes, bounding box and visibility
- **Wait for Element**: Block until an element is visible, hidden, present, or absent
- **Mouse Actions**: Click, double-click, hover, right-click, and drag
- **Fill Text Fields**: Fill input and textarea elements
- **Check Elements**: Check/uncheck checkboxes and select radio buttons
- **Press Key**: Simulate key presses (Enter, Escape, Tab, etc.)
- **Upload Files**: Set files on file inputs (works on hidden inputs)
- **Download Files**: Click download links/buttons and save files
- **Select Dropdown**: Choose options by value, label, or index
- **Console Logs**: Capture browser console messages and errors
- **Network Monitor**: Track HTTP requests with filtering and body inspection
- **Request Interception**: Block, redirect, and modify in-flight requests, or mock responses
- **HTML Extraction**: Get page HTML with optional regex filtering
- **Screenshots**: Viewport or full-page screenshots saved to `/tmp`
- **PDF Export**: Save the current page as a PDF with format, margin, and header/footer options
- **Scroll**: Scroll to an element, an X/Y position, or the top/bottom of the page
- **Resize**: Set the viewport size
- **Cookie Management**: List and clear cookies per tab or all origins
- **DOM Storage**: Inspect and clear localStorage / sessionStorage
- **Clear Browser Data**: Wipe cache, cookies, IndexedDB, service workers, and more
- **Tab Management**: List (with active tab indicator), activate, close, and refresh tabs
- **Self-Update**: Replace the binary and docs in place from the latest release (`update`, fork-aware)

## Quick Start

Build the binary:

```bash
go build -o scripts/browser-tools .
```

And copy the whole directory to your skills directory. Or download a prebuilt one from releases.

Then mention "use the browser-tools skill" and the agent will invoke it automatically.

## Documentation

- **[SKILL.md](SKILL.md)** — Quick reference for agents
- **[REFERENCE.md](REFERENCE.md)** — Full command documentation

## Requirements

- Go 1.21+
- Chrome/Chromium browser

## User Profile

Remote debugging is not allowed in the main Chrome profile, so a separate one is created and reused per variant:

- `~/.cache/claude-browser-tools/<variant>/`
