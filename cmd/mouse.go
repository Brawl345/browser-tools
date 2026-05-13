package cmd

import (
	"context"
	"fmt"
	"os"

	"browser-tools/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Mouse(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("mouse", flag.ExitOnError)
	to := fs.String("to", "", "target CSS selector for drag")
	force := fs.Bool("force", false, "dispatch JS events even if element is not visible")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools mouse <action> [--to <selector>] [--force] <selector>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Actions: click, dblclick, hover, right-click, drag")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() < 2 {
		fs.Usage()
		os.Exit(1)
	}

	action, sel := fs.Arg(0), fs.Arg(1)

	if action == "drag" && *to == "" {
		fmt.Fprintln(os.Stderr, "error: drag requires --to <selector>")
		os.Exit(1)
	}

	if err := browser.EnsureRunning(variant, port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	allocCtx, _ := browser.Connect(ctx, port)
	tabCtx, _ := browser.ExistingOrNewTab(allocCtx)

	var err error
	if *force {
		err = mouseForce(tabCtx, action, sel, *to)
	} else {
		err = mouseNative(tabCtx, action, sel, *to)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		if !*force {
			fmt.Fprintf(os.Stderr, "hint: use --force if the element is not visible\n")
		}
		os.Exit(1)
	}

	fmt.Printf("%s✓%s %s: %s\n", ansiGreen, ansiReset, action, sel)
}

func mouseNative(ctx context.Context, action, sel, to string) error {
	switch action {
	case "click":
		return chromedp.Run(ctx, chromedp.Click(sel, chromedp.ByQuery))

	case "dblclick":
		return chromedp.Run(ctx, chromedp.DoubleClick(sel, chromedp.ByQuery))

	case "right-click":
		return chromedp.Run(ctx, chromedp.QueryAfter(sel,
			func(ctx context.Context, _ cdpruntime.ExecutionContextID, nodes ...*cdp.Node) error {
				if len(nodes) == 0 {
					return fmt.Errorf("element not found: %s", sel)
				}
				return chromedp.MouseClickNode(nodes[0], chromedp.ButtonRight).Do(ctx)
			}, chromedp.ByQuery, chromedp.NodeVisible))

	case "hover":
		return chromedp.Run(ctx, chromedp.QueryAfter(sel,
			func(ctx context.Context, _ cdpruntime.ExecutionContextID, nodes ...*cdp.Node) error {
				if len(nodes) == 0 {
					return fmt.Errorf("element not found: %s", sel)
				}
				x, y, err := nodeCenter(ctx, nodes[0])
				if err != nil {
					return err
				}
				return chromedp.MouseEvent(input.MouseMoved, x, y).Do(ctx)
			}, chromedp.ByQuery, chromedp.NodeVisible))

	case "drag":
		return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
			srcX, srcY, err := nodeCenterBySelector(ctx, sel)
			if err != nil {
				return fmt.Errorf("source: %w", err)
			}
			dstX, dstY, err := nodeCenterBySelector(ctx, to)
			if err != nil {
				return fmt.Errorf("target: %w", err)
			}
			return dragXY(ctx, srcX, srcY, dstX, dstY)
		}))

	default:
		return fmt.Errorf("unknown action %q (use: click, dblclick, hover, right-click, drag)", action)
	}
}

func mouseForce(ctx context.Context, action, sel, to string) error {
	var script string
	switch action {
	case "click":
		script = fmt.Sprintf(`(function(){
			const el = document.querySelector(%s);
			if (!el) return false;
			el.dispatchEvent(new MouseEvent('mousedown', {bubbles:true, cancelable:true, button:0}));
			el.dispatchEvent(new MouseEvent('mouseup',   {bubbles:true, cancelable:true, button:0}));
			el.dispatchEvent(new MouseEvent('click',     {bubbles:true, cancelable:true, button:0}));
			return true;
		})()`, jsonStr(sel))
	case "dblclick":
		script = fmt.Sprintf(`(function(){
			const el = document.querySelector(%s);
			if (!el) return false;
			el.dispatchEvent(new MouseEvent('dblclick', {bubbles:true, cancelable:true, button:0}));
			return true;
		})()`, jsonStr(sel))
	case "hover":
		script = fmt.Sprintf(`(function(){
			const el = document.querySelector(%s);
			if (!el) return false;
			el.dispatchEvent(new MouseEvent('mouseover',  {bubbles:true}));
			el.dispatchEvent(new MouseEvent('mouseenter', {bubbles:false}));
			el.dispatchEvent(new MouseEvent('mousemove',  {bubbles:true}));
			return true;
		})()`, jsonStr(sel))
	case "right-click":
		script = fmt.Sprintf(`(function(){
			const el = document.querySelector(%s);
			if (!el) return false;
			el.dispatchEvent(new MouseEvent('contextmenu', {bubbles:true, cancelable:true, button:2}));
			return true;
		})()`, jsonStr(sel))
	case "drag":
		script = fmt.Sprintf(`(function(){
			const src = document.querySelector(%s);
			const dst = document.querySelector(%s);
			if (!src || !dst) return false;
			const dt = new DataTransfer();
			src.dispatchEvent(new DragEvent('dragstart', {bubbles:true, cancelable:true, dataTransfer:dt}));
			dst.dispatchEvent(new DragEvent('dragenter', {bubbles:true, cancelable:true, dataTransfer:dt}));
			dst.dispatchEvent(new DragEvent('dragover',  {bubbles:true, cancelable:true, dataTransfer:dt}));
			dst.dispatchEvent(new DragEvent('drop',      {bubbles:true, cancelable:true, dataTransfer:dt}));
			src.dispatchEvent(new DragEvent('dragend',   {bubbles:true, cancelable:true, dataTransfer:dt}));
			return true;
		})()`, jsonStr(sel), jsonStr(to))
	default:
		return fmt.Errorf("unknown action %q (use: click, dblclick, hover, right-click, drag)", action)
	}

	var ok bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &ok)); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("element not found: %s", sel)
	}
	return nil
}

func nodeCenter(ctx context.Context, n *cdp.Node) (float64, float64, error) {
	if err := dom.ScrollIntoViewIfNeeded().WithNodeID(n.NodeID).Do(ctx); err != nil {
		return 0, 0, err
	}
	boxes, err := dom.GetContentQuads().WithNodeID(n.NodeID).Do(ctx)
	if err != nil {
		return 0, 0, err
	}
	if len(boxes) == 0 {
		return 0, 0, chromedp.ErrInvalidDimensions
	}
	q := boxes[0]
	var x, y float64
	for i := 0; i < len(q); i += 2 {
		x += q[i]
		y += q[i+1]
	}
	n2 := float64(len(q) / 2)
	return x / n2, y / n2, nil
}

func nodeCenterBySelector(ctx context.Context, sel string) (float64, float64, error) {
	var x, y float64
	err := chromedp.Run(ctx, chromedp.QueryAfter(sel,
		func(ctx context.Context, _ cdpruntime.ExecutionContextID, nodes ...*cdp.Node) error {
			if len(nodes) == 0 {
				return fmt.Errorf("element not found: %s", sel)
			}
			var err error
			x, y, err = nodeCenter(ctx, nodes[0])
			return err
		}, chromedp.ByQuery, chromedp.NodeVisible))
	return x, y, err
}

func dragXY(ctx context.Context, srcX, srcY, dstX, dstY float64) error {
	// mousedown at source
	if err := (input.DispatchMouseEvent(input.MousePressed, srcX, srcY).
		WithButton(input.Left).WithClickCount(1)).Do(ctx); err != nil {
		return err
	}
	// interpolate mouse moves from src to dst
	steps := 10
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		mx := srcX + (dstX-srcX)*t
		my := srcY + (dstY-srcY)*t
		if err := (input.DispatchMouseEvent(input.MouseMoved, mx, my).
			WithButton(input.Left).WithButtons(1)).Do(ctx); err != nil {
			return err
		}
	}
	// mouseup at destination
	return (input.DispatchMouseEvent(input.MouseReleased, dstX, dstY).
		WithButton(input.Left).WithClickCount(1)).Do(ctx)
}

