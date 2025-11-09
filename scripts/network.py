#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "click",
#     "playwright",
#     "rich",
# ]
# ///

import asyncio
import click
import re
from rich.console import Console
from rich.table import Table
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
@click.option(
    "--no-reload",
    is_flag=True,
    help="Don't reload the page, wait for requests (10s timeout)"
)
@click.option(
    "--duration",
    type=int,
    default=10,
    help="Duration in seconds to capture requests when using --no-reload (default: 10)"
)
def main(port, resource_type, show_headers, show_body, url_filter, no_reload, duration):
    """Capture network requests from an existing Chrome instance."""
    asyncio.run(capture_network(port, resource_type, show_headers, show_body, url_filter, no_reload, duration))

async def capture_network(port, resource_type, show_headers, show_body, url_filter, no_reload, duration):
    async with async_playwright():
        try:
            browser, page = await get_browser_and_page(port)
            if not browser or not page:
                return

            url_pattern = re.compile(url_filter) if url_filter else None
            requests_data = []

            def handle_request(request: Request):
                if resource_type != "all" and request.resource_type != resource_type:
                    return

                if url_pattern and not url_pattern.search(request.url):
                    return

                req_data = {
                    "method": request.method,
                    "url": request.url,
                    "type": request.resource_type,
                    "headers": dict(request.headers) if show_headers else None,
                    "post_data": request.post_data if show_body and request.resource_type in ["fetch", "xhr"] else None,
                    "status": None,
                    "response_headers": None,
                    "response_body": None,
                    "request_obj": request
                }
                requests_data.append(req_data)

            async def handle_response(response: Response):
                for req_data in requests_data:
                    if req_data["url"] == response.url and req_data["status"] is None:
                        req_data["status"] = response.status
                        if show_headers:
                            req_data["response_headers"] = await response.all_headers()
                        if show_body and req_data["type"] in ["fetch", "xhr"]:
                            try:
                                req_data["response_body"] = await response.text()
                            except:
                                try:
                                    req_data["response_body"] = f"<binary data, {len(await response.body())} bytes>"
                                except:
                                    req_data["response_body"] = "<failed to read>"
                        break

            page.on("request", handle_request)
            page.on("response", handle_response)

            if no_reload:
                console.print(f"[bold cyan]Listening for network requests for {duration} seconds...[/bold cyan]")
                await asyncio.sleep(duration)
            else:
                console.print("[bold cyan]Reloading page to capture network requests...[/bold cyan]")
                await page.reload(wait_until="networkidle")

            if not requests_data:
                console.print("[dim]No network requests captured.[/dim]")
            else:
                table = Table(show_header=True, header_style="bold magenta")
                table.add_column("Method", style="cyan")
                table.add_column("Status", style="green")
                table.add_column("Type", style="yellow")
                table.add_column("URL")

                for req in requests_data:
                    status = str(req["status"]) if req["status"] else "-"
                    status_style = "green" if req["status"] and 200 <= req["status"] < 300 else "red"

                    table.add_row(
                        req["method"],
                        f"[{status_style}]{status}[/{status_style}]",
                        req["type"],
                        req["url"][:100]
                    )

                console.print(table)
                console.print(f"\n[bold]Total requests: {len(requests_data)}[/bold]")

                if show_headers or show_body:
                    console.print("\n[bold cyan]Request/Response Details:[/bold cyan]")
                    for i, req in enumerate(requests_data, 1):
                        if show_body and req["type"] not in ["fetch", "xhr"]:
                            continue

                        console.print(f"\n[bold]{i}. {req['method']} {req['url'][:80]}[/bold]")

                        if show_headers and req["headers"]:
                            console.print("  [yellow]Request Headers:[/yellow]")
                            for key, value in req["headers"].items():
                                console.print(f"    {key}: {value[:100]}")

                        if show_body and req["post_data"]:
                            console.print("  [yellow]Request Body:[/yellow]")
                            console.print(f"    {req['post_data'][:500]}")

                        if show_headers and req["response_headers"]:
                            console.print("  [green]Response Headers:[/green]")
                            for key, value in req["response_headers"].items():
                                console.print(f"    {key}: {value[:100]}")

                        if show_body and req["response_body"]:
                            console.print("  [green]Response Body:[/green]")
                            console.print(f"    {req['response_body'][:500]}")

        except Exception as e:
            click.echo(f"Failed to connect or capture: {e}", err=True)

if __name__ == "__main__":
    main()
