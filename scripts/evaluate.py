import asyncio
import click
from pathlib import Path
from playwright.async_api import async_playwright
from browser_utils import get_browser_and_page

@click.command()
@click.argument("javascript")
@click.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
def main(javascript, port):
    """Execute JavaScript in an existing Chrome instance.

    JAVASCRIPT can be either inline code or a path to a .js file.
    """
    js_path = Path(javascript)
    if js_path.is_file():
        javascript = js_path.read_text()

    asyncio.run(evaluate_js(javascript, port))

async def evaluate_js(javascript, port):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            result = await page.evaluate(javascript)
            if result is not None:
                click.echo(result)
        except Exception as e:
            click.echo(f"Failed to connect or evaluate: {e}", err=True)

if __name__ == "__main__":
    main()
