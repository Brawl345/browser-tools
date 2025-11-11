# Browser Tools

A Claude Code skill for browser automation using Playwright. Requires `uv`. And yes, this was coded by Claude Code. Inspired by [this blogpost](https://mariozechner.at/posts/2025-11-02-what-if-you-dont-need-mcp/).

## Features

- **Start Browser**: Launch Chrome with remote debugging enabled
- **Navigate**: Open URLs in active or new tabs
- **Execute JavaScript**: Run inline code or scripts from files
- **Element Picker**: Interactive DOM element selection
- **Click Element**: Click on elements using CSS selectors
- **Fill Text Fields**: Fill input and textarea elements with text
- **Check Elements**: Check/uncheck checkboxes and select radio buttons
- **Press Key**: Press keyboard keys (Enter, Escape, Tab, etc.)
- **Select Dropdown**: Choose options from dropdown menus
- **Console Logs**: Capture browser console messages and errors
- **Network Monitor**: Track HTTP requests with filtering and body inspection
- **HTML Extraction**: Get page HTML with optional context search
- **Screenshots**: Capture timestamped screenshots
- **Cookie Access**: List all cookies for the current site

## Quick Start

Clone this directory into your [Claude skills directory](https://code.claude.com/docs/en/skills). I won't bother making a plugin.

Then just mention "use the browser-tools skill to do XY" and Claude should invoke it automatically. It's recommended to set `CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR` to `1` in your [environment](https://code.claude.com/docs/en/settings#environment-variables).

## Documentation

- **[SKILL.md](SKILL.md)** - Quick reference for Claude
- **[REFERENCE.md](REFERENCE.md)** - Complete API documentation, read by Claude when needing more context

## Requirements

- Python 3.11+
- uv package manager
- Chrome/Chromium browser

All dependencies are automatically installed via uv's inline script metadata.

## Platform Support

- macOS: Chrome variants in `/Applications/`
- Windows: Chrome in `%PROGRAMFILES%` or `%LOCALAPPDATA%`
- Linux: Chrome via `which` or `/usr/bin/`

## User Profile Location

Remote debugging is not allowed in the main Chrome profile so a new one is created and re-used everytime. It's stored
here, per Chrome variant:

- macOS/Linux: `~/.cache/claude-browser-tools/<variant>/`
- Windows: `%LOCALAPPDATA%\claude-browser-tools\<variant>\`
