#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "click",
#     "playwright @ git+https://github.com/microsoft/playwright-python.git@f283da50d64ee896d0a148522b6c719b86b721f3",
#     "rich",
# ]
# ///

import asyncio
import click
from rich.console import Console
from rich.style import Style
from playwright.async_api import async_playwright, ConsoleMessage
from browser_utils import get_browser_and_page

console = Console()

TYPE_STYLES = {
    "log": Style(color="white"),
    "info": Style(color="cyan"),
    "warning": Style(color="yellow"),
    "error": Style(color="red", bold=True),
    "debug": Style(color="blue"),
}

@click.command()
@click.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
@click.option(
    "--errors-only",
    is_flag=True,
    help="Only show errors and warnings"
)
def main(port, errors_only):
    """Get console messages from an existing Chrome instance."""
    asyncio.run(get_console_messages(port, errors_only))

async def get_console_messages(port, errors_only):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            messages = await page.console_messages()

            if not messages:
                console.print("[dim]No console messages available.[/dim]")
            else:
                for msg in messages:
                    msg_type = msg.type

                    if errors_only and msg_type not in ["error", "warning"]:
                        continue

                    style = TYPE_STYLES.get(msg_type, Style())
                    prefix = f"[{msg_type.upper()}]"
                    console.print(f"{prefix} {msg.text}", style=style)

                    if msg.location:
                        loc = msg.location
                        console.print(f"  at {loc.get('url', '')}:{loc.get('lineNumber', '')}:{loc.get('columnNumber', '')}", style=Style(dim=True))

        except Exception as e:
            click.echo(f"Failed to connect: {e}", err=True)

if __name__ == "__main__":
    main()
