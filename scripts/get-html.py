#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "click",
#     "playwright",
# ]
# ///

import asyncio
import click
from playwright.async_api import async_playwright
from browser_utils import get_browser_and_page

@click.command()
@click.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
@click.option(
    "--context",
    type=str,
    help="Search string to find and show surrounding lines"
)
@click.option(
    "--lines",
    type=int,
    default=5,
    help="Number of lines before and after context match (default: 5)"
)
def main(port, context, lines):
    """Get the HTML content of the current page."""
    asyncio.run(get_html(port, context, lines))

async def get_html(port, context, context_lines):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            html = await page.content()

            if context:
                lines = html.split('\n')
                matches = []
                for i, line in enumerate(lines):
                    if context in line:
                        start = max(0, i - context_lines)
                        end = min(len(lines), i + context_lines + 1)
                        matches.append({
                            'line_num': i + 1,
                            'lines': lines[start:end],
                            'start': start + 1
                        })

                if not matches:
                    click.echo(f"No matches found for '{context}'", err=True)
                    return

                for match in matches:
                    click.echo(f"--- Match at line {match['line_num']} ---")
                    for idx, line in enumerate(match['lines'], start=match['start']):
                        prefix = ">>> " if idx == match['line_num'] else "    "
                        click.echo(f"{prefix}{idx}: {line}")
                    click.echo()
            else:
                click.echo(html)
        except Exception as e:
            click.echo(f"Failed to connect or get HTML: {e}", err=True)

if __name__ == "__main__":
    main()
