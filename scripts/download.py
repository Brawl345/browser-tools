import asyncio
import click as click_lib
from pathlib import Path
from playwright.async_api import async_playwright
from browser_utils import get_browser_and_page

@click_lib.command()
@click_lib.argument("selector")
@click_lib.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
@click_lib.option(
    "--timeout",
    type=int,
    default=30000,
    help="Timeout in milliseconds (default: 30000)"
)
@click_lib.option(
    "--output",
    type=str,
    help="Output path for downloaded file (default: downloads directory)"
)
def main(selector, port, timeout, output):
    """Click a download link/button and save the downloaded file.

    Example:
      download.py "a[href='/report.pdf']"
      download.py "button#download" --output ~/Downloads/report.pdf
      download.py ".download-button" --timeout 60000
    """
    asyncio.run(download_file(selector, port, timeout, output))

async def download_file(selector, port, timeout, output):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            await page.bring_to_front()

            click_lib.echo(f"Looking for download element: {selector}")

            async with page.expect_download(timeout=timeout) as download_info:
                await page.locator(selector).click(timeout=timeout)

            download = await download_info.value

            if output:
                output_path = Path(output).expanduser().resolve()
                output_path.parent.mkdir(parents=True, exist_ok=True)
                await download.save_as(str(output_path))
                click_lib.echo(f"Downloaded to: {output_path}")
            else:
                suggested_filename = download.suggested_filename
                default_path = Path.home() / "Downloads" / suggested_filename
                default_path.parent.mkdir(parents=True, exist_ok=True)
                await download.save_as(str(default_path))
                click_lib.echo(f"Downloaded to: {default_path}")

        except TimeoutError:
            click_lib.echo(f"Timeout: No download started within {timeout}ms", err=True)
        except Exception as e:
            click_lib.echo(f"Failed to download file: {e}", err=True)

if __name__ == "__main__":
    main()
