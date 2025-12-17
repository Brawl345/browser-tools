import asyncio
import click
import re
from rich.console import Console
from playwright.async_api import async_playwright, Request, Response
from browser_utils import get_browser_and_page

console = Console()

@click.command()
@click.option(
    "--port",
    type=int,
    default=9222,
    help="Remote debugging port (default: 9222)"
)
@click.option(
    "--type",
    "resource_type",
    type=click.Choice(["all", "xhr", "fetch", "document", "script", "stylesheet", "image", "font", "media"]),
    default="all",
    help="Filter by resource type (default: all)"
)
@click.option(
    "--show-headers",
    is_flag=True,
    help="Show request and response headers"
)
@click.option(
    "--show-body",
    is_flag=True,
    help="Show request and response bodies (only for fetch/xhr)"
)
@click.option(
    "--filter",
    "url_filter",
    type=str,
    default=None,
    help="Filter URLs by regex pattern"
)
def main(port, resource_type, show_headers, show_body, url_filter):
    """Capture network requests from an existing Chrome instance."""
    asyncio.run(capture_network(port, resource_type, show_headers, show_body, url_filter))

async def capture_network(port, resource_type, show_headers, show_body, url_filter):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            url_pattern = re.compile(url_filter) if url_filter else None
            request_count = 0

            def handle_request(request: Request):
                nonlocal request_count

                if resource_type != "all" and request.resource_type != resource_type:
                    return

                if url_pattern and not url_pattern.search(request.url):
                    return

                request_count += 1

                console.print(f"\n[bold cyan]→ REQUEST #{request_count}[/bold cyan]")
                console.print(f"  [cyan]{request.method:6}[/cyan] [{request.resource_type:10}] {request.url}")

                if show_headers:
                    console.print("  [yellow]Request Headers:[/yellow]")
                    for key, value in request.headers.items():
                        console.print(f"    {key}: {value}")

                if show_body and request.resource_type in ["fetch", "xhr"] and request.post_data:
                    console.print("  [yellow]Request Body:[/yellow]")
                    console.print(f"    {request.post_data}")

            async def handle_response(response: Response):
                if resource_type != "all" and response.request.resource_type != resource_type:
                    return

                if url_pattern and not url_pattern.search(response.url):
                    return

                status_style = "green" if 200 <= response.status < 300 else "red"
                console.print(f"[bold {status_style}]← RESPONSE[/bold {status_style}]")
                console.print(f"  [{status_style}]{response.status}[/{status_style}] {response.url}")

                if show_headers:
                    headers = await response.all_headers()
                    console.print("  [green]Response Headers:[/green]")
                    for key, value in headers.items():
                        console.print(f"    {key}: {value}")

                if show_body and response.request.resource_type in ["fetch", "xhr"]:
                    console.print("  [green]Response Body:[/green]")
                    try:
                        body = await response.text()
                        console.print(f"    {body}")
                    except Exception:
                        try:
                            body_bytes = await response.body()
                            console.print(f"    <binary data, {len(body_bytes)} bytes>")
                        except Exception:
                            console.print("    <failed to read>")

            page.on("request", handle_request)
            page.on("response", handle_response)

            console.print("[bold cyan]Listening for network requests (press Ctrl+C to stop)...[/bold cyan]")

            try:
                while True:
                    await asyncio.sleep(1)
            except KeyboardInterrupt:
                console.print(f"\n\n[bold cyan]Stopped. Total requests: {request_count}[/bold cyan]")

        except Exception as e:
            click.echo(f"Failed to connect or capture: {e}", err=True)

if __name__ == "__main__":
    main()
