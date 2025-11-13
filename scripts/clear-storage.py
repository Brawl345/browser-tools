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
    "--local",
    is_flag=True,
    help="Clear localStorage only"
)
@click.option(
    "--session",
    is_flag=True,
    help="Clear sessionStorage only"
)
def main(port, local, session):
    """Clear localStorage and/or sessionStorage from an existing Chrome instance.

    By default, clears both localStorage and sessionStorage for the current page.
    """
    if local and not session:
        storage_type = "local"
    elif session and not local:
        storage_type = "session"
    else:
        storage_type = "all"

    asyncio.run(clear_storage(port, storage_type))

async def clear_storage(port, storage_type):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            cleared = []

            if storage_type in ["local", "all"]:
                await page.evaluate("localStorage.clear()")
                cleared.append("localStorage")

            if storage_type in ["session", "all"]:
                await page.evaluate("sessionStorage.clear()")
                cleared.append("sessionStorage")

            click.echo(f"Cleared {' and '.join(cleared)} for {page.url}")

        except Exception as e:
            click.echo(f"Failed to clear storage: {e}", err=True)

if __name__ == "__main__":
    main()
