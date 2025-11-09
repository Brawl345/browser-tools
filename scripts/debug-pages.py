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

@click.command()
@click.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
def main(port):
    """Debug: Show all pages in the browser context."""
    asyncio.run(debug_pages(port))

async def debug_pages(port):
    async with async_playwright() as p:
        try:
            browser = await p.chromium.connect_over_cdp(f"http://localhost:{port}")
            contexts = browser.contexts

            if not contexts:
                click.echo("No browser contexts found")
                return

            click.echo(f"Found {len(contexts)} context(s)\n")

            for ctx_idx, context in enumerate(contexts):
                click.echo(f"Context {ctx_idx}:")
                pages = context.pages
                click.echo(f"  Total pages: {len(pages)}\n")

                for idx, page in enumerate(pages):
                    click.echo(f"  [{idx}] {page.url}")
                    click.echo(f"      Title: {await page.title()}")
                    click.echo(f"      Is chrome://: {page.url.startswith('chrome://')}")
                    click.echo()

        except Exception as e:
            click.echo(f"Failed to connect: {e}", err=True)

if __name__ == "__main__":
    main()
