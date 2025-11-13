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
from browser_utils import get_browser_and_page, get_context

@click.command()
@click.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
@click.option(
    "--all",
    "clear_all",
    is_flag=True,
    help="Clear all cookies from all origins (default: only current page)"
)
def main(port, clear_all):
    """Clear cookies from an existing Chrome instance.

    By default, clears cookies for the current page only.
    Use --all to clear all cookies from all origins.
    """
    asyncio.run(clear_cookies(port, clear_all))

async def clear_cookies(port, clear_all):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            context = await get_context(browser)
            if not context:
                return

            if clear_all:
                await context.clear_cookies()
                click.echo("All cookies cleared")
            else:
                current_url = page.url
                cookies = await context.cookies(current_url)

                if not cookies:
                    click.echo("No cookies to clear")
                    return

                cookie_names = [c['name'] for c in cookies]
                await context.clear_cookies()
                click.echo(f"Cleared {len(cookies)} cookie(s) for {current_url}")

        except Exception as e:
            click.echo(f"Failed to clear cookies: {e}", err=True)

if __name__ == "__main__":
    main()
