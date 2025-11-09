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
from datetime import datetime
from pathlib import Path
from tempfile import gettempdir
from playwright.async_api import async_playwright
from browser_utils import get_browser_and_page

@click.command()
@click.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
def main(port):
    """Take a screenshot of the current page in an existing Chrome instance."""
    asyncio.run(take_screenshot(port))

async def take_screenshot(port):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
            screenshot_path = Path(gettempdir()) / f"screenshot_{timestamp}.png"

            await page.screenshot(path=str(screenshot_path))
            click.echo(f"Screenshot saved to {screenshot_path}")
        except Exception as e:
            click.echo(f"Failed to connect or take screenshot: {e}", err=True)

if __name__ == "__main__":
    main()
