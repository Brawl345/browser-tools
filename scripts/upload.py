import asyncio
import click as click_lib
from pathlib import Path
from playwright.async_api import async_playwright
from browser_utils import get_browser_and_page

@click_lib.command()
@click_lib.argument("selector")
@click_lib.argument("file_paths", nargs=-1, required=True)
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
def main(selector, file_paths, port, timeout):
    """Upload files to a file input using a CSS selector.

    Example:
      upload.py "input[type='file']" /path/to/file.pdf
      upload.py "#file-upload" /path/to/image1.jpg /path/to/image2.png
      upload.py "input[name='document']" ~/Documents/report.pdf --timeout 60000
    """
    asyncio.run(upload_files(selector, file_paths, port, timeout))

async def upload_files(selector, file_paths, port, timeout):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            await page.bring_to_front()

            click_lib.echo(f"Looking for file input: {selector}")

            absolute_paths = []
            for file_path in file_paths:
                path = Path(file_path).expanduser().resolve()
                if not path.exists():
                    click_lib.echo(f"File not found: {file_path}", err=True)
                    return
                absolute_paths.append(str(path))

            locator = page.locator(selector)

            await locator.set_input_files(absolute_paths, timeout=timeout)

            if len(absolute_paths) == 1:
                click_lib.echo(f"Successfully uploaded: {absolute_paths[0]}")
            else:
                click_lib.echo(f"Successfully uploaded {len(absolute_paths)} files")
                for path in absolute_paths:
                    click_lib.echo(f"  - {path}")

        except Exception as e:
            click_lib.echo(f"Failed to upload files: {e}", err=True)

if __name__ == "__main__":
    main()
