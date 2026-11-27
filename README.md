# Browser Tools

A coding agent skill for browser automation using Go and the Chrome DevTools Protocol. And yes, this was coded by an AI. Inspired by [this blogpost](https://mariozechner.at/posts/2025-11-02-what-if-you-dont-need-mcp/).

## Features

- **Start Browser**: Launch Chrome with remote debugging, selectable via `--browser` or `BROWSER_TOOLS_BROWSER` (`chrome-stable`, `chrome-beta`, `chrome-dev`, `chrome-canary`, `cft-stable`, `cft-beta`, `cft-dev`, `cft-canary`), optionally with a fake camera and microphone (`--fake-media`) or any extra Chrome flag (`--arg`), and quit it again (`stop`)
- **Chrome for Testing**: Download and update Chrome for Testing builds per channel (`update-cft`)
- **Extensions**: Load, list, uninstall, and trigger the toolbar action of unpacked extensions on Chrome for Testing
- **Navigate**: Open URLs in the active or a new tab, waiting for the page to load
- **Reload**: Reload the active tab, optionally bypassing the cache
- **Execute JavaScript**: Run inline code, files, or STDIN input
- **Element Picker**: Interactive DOM element selection
- **CSS, XPath or text**: Every selector argument takes a CSS selector, an XPath expression, or `text=Some Label`, which matches visible text, button values, ARIA labels and placeholders, chained with `>>` to search inside a match
- **Shadow DOM**: CSS and text selectors reach into open shadow roots
- **Element Info**: Read a selector's text, value, attributes, bounding box, computed style and visibility
- **Wait for Element**: Block until an element is visible, hidden, present, or absent, or until the page goes idle
- **Failure Reports**: A selector that does not resolve names the tab, says whether the element was missing, invisible or covered, and lists what is on the page instead
- **Dialogs**: Answer alert, confirm, prompt and beforeunload dialogs, accepting them unless told otherwise
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
- **Screenshots**: Viewport, full-page or single-element screenshots, to `/tmp` or a path of your choice
- **PDF Export**: Save the current page as a PDF with format, margin, and header/footer options
- **Scroll**: Scroll to an element, an X/Y position, or the top/bottom of the page
- **Resize**: Emulate a viewport size, a touch phone or a dark color scheme until reset
- **Cookie Management**: List and clear cookies per tab or all origins
- **DOM Storage**: Inspect and clear localStorage / sessionStorage
- **Clear Browser Data**: Wipe cache, cookies, IndexedDB, service workers, and more
- **Tab Management**: List, activate, close, and refresh tabs; popups and dialogs opening in front become the tab commands act on
- **Self-Update**: Replace the binary and docs in place from the latest release (`update`, fork-aware)

## Quick Start

Build the binary:

```bash
go build -o scripts/browser-tools .
```

And copy the whole directory to your skills directory. Or download a prebuilt one from releases.

Then mention "use the browser-tools skill" and the agent will invoke it automatically.

### Nix

Build the binary with the flake:

```bash
nix build github:Brawl345/browser-tools
```

The `browser-tools-skill` package bundles `SKILL.md`, `REFERENCE.md`, and the binary under `scripts/` into a ready-to-use skill directory. Install it declaratively with [home-manager](https://nix-community.github.io/home-manager/options/home-manager/programs/claude-code.html#opt-programs.claude-code.skills):

```nix
{
  inputs.browser-tools.url = "github:Brawl345/browser-tools";

  # in your home-manager config:
  programs.claude-code.skills.browser-tools =
    "${inputs.browser-tools.packages.${pkgs.system}.browser-tools-skill}";
}
```

This symlinks the skill into `~/.claude/skills/browser-tools/`.

## Documentation

- **[SKILL.md](SKILL.md)**: Quick reference for agents
- **[REFERENCE.md](REFERENCE.md)**: Full command documentation

## Requirements

- Go 1.26+
- Chrome/Chromium browser

## User Profile

Remote debugging is not allowed in the main Chrome profile, so a separate one is created and reused per variant:

- `~/.cache/claude-browser-tools/<variant>/`

Chrome for Testing builds are downloaded to:

- `~/.cache/claude-browser-tools/chrome-for-testing-binaries/<channel>/`
