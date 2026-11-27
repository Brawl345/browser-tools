package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"browser-tools/browser"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

func Mouse(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("mouse", flag.ExitOnError)
	to := fs.String("to", "", "target selector for drag")
	force := fs.Bool("force", false, "dispatch JS events even if the element is not visible")
	forceAvailable = true
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
		fail("drag requires --to <selector>")
	}

	s := MustSession(ctx, variant, port)

	want := StateVisible
	if *force {
		want = StatePresent
	}
	m := Locate(ctx, s, sel, want)
	settle := SettleAfter(m.Tab)

	var err error
	if *force {
		err = mouseForce(m.Tab, action, *to)
	} else {
		err = mouseNative(ctx, s, m, action, *to)
	}
	if err != nil {
		fail("%v", err)
	}

	settle()
	fmt.Printf("%s✓%s %s: %s%s\n", ansiGreen, ansiReset, action, sel, pointerNote(m))
}

// matchNote warns when a selector was ambiguous, so the caller knows the action
// hit the first of several elements.
func matchNote(m *Match) string {
	if m.Count > 1 {
		return " (first of " + fmt.Sprint(m.Count) + " matches)"
	}
	return ""
}

// pointerNote adds what lies on top of the element, since that is what a click
// at its centre really reaches.
func pointerNote(m *Match) string {
	var parts []string
	if m.Count > 1 {
		parts = append(parts, fmt.Sprintf("first of %d matches", m.Count))
	}
	if m.Covered != "" {
		parts = append(parts, fmt.Sprintf("%s lies on top and takes the event", m.Covered))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, "; ") + ")"
}

func mouseNative(ctx context.Context, s *browser.Session, m *Match, action, to string) error {
	switch action {
	case "click":
		return PointerAt(m.Tab, m.X, m.Y, input.Left, 1)
	case "dblclick":
		return PointerAt(m.Tab, m.X, m.Y, input.Left, 2)
	case "right-click":
		return PointerAt(m.Tab, m.X, m.Y, input.Right, 1)
	case "hover":
		return PointerAt(m.Tab, m.X, m.Y, input.None, 0)
	case "drag":
		dst := Locate(ctx, s, to, StateVisible)
		return dragXY(m.Tab, m.X, m.Y, dst.X, dst.Y)
	default:
		return fmt.Errorf("unknown action %q (use: click, dblclick, hover, right-click, drag)", action)
	}
}

func mouseForce(tabCtx context.Context, action, to string) error {
	var body string
	switch action {
	case "click":
		body = `el.dispatchEvent(new MouseEvent('mousedown', {bubbles:true, cancelable:true, button:0}));
			el.dispatchEvent(new MouseEvent('mouseup',   {bubbles:true, cancelable:true, button:0}));
			el.click();
			return true;`
	case "dblclick":
		body = `el.dispatchEvent(new MouseEvent('dblclick', {bubbles:true, cancelable:true, button:0})); return true;`
	case "hover":
		body = `el.dispatchEvent(new MouseEvent('mouseover',  {bubbles:true}));
			el.dispatchEvent(new MouseEvent('mouseenter', {bubbles:false}));
			el.dispatchEvent(new MouseEvent('mousemove',  {bubbles:true}));
			return true;`
	case "right-click":
		body = `el.dispatchEvent(new MouseEvent('contextmenu', {bubbles:true, cancelable:true, button:2})); return true;`
	case "drag":
		body = fmt.Sprintf(`const dst = document.querySelector(%s);
			if (!dst) return false;
			const dt = new DataTransfer();
			el.dispatchEvent(new DragEvent('dragstart', {bubbles:true, cancelable:true, dataTransfer:dt}));
			dst.dispatchEvent(new DragEvent('dragenter', {bubbles:true, cancelable:true, dataTransfer:dt}));
			dst.dispatchEvent(new DragEvent('dragover',  {bubbles:true, cancelable:true, dataTransfer:dt}));
			dst.dispatchEvent(new DragEvent('drop',      {bubbles:true, cancelable:true, dataTransfer:dt}));
			el.dispatchEvent(new DragEvent('dragend',   {bubbles:true, cancelable:true, dataTransfer:dt}));
			return true;`, jsonStr(to))
	default:
		return fmt.Errorf("unknown action %q (use: click, dblclick, hover, right-click, drag)", action)
	}

	var ok bool
	if err := OnHit(tabCtx, body, &ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("the element did not accept the %s", action)
	}
	return nil
}

func dragXY(tabCtx context.Context, srcX, srcY, dstX, dstY float64) error {
	return RunBounded(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		if err := input.DispatchMouseEvent(input.MousePressed, srcX, srcY).
			WithButton(input.Left).WithClickCount(1).Do(ctx); err != nil {
			return err
		}
		const steps = 10
		for i := 1; i <= steps; i++ {
			t := float64(i) / steps
			if err := input.DispatchMouseEvent(input.MouseMoved, srcX+(dstX-srcX)*t, srcY+(dstY-srcY)*t).
				WithButton(input.Left).WithButtons(1).Do(ctx); err != nil {
				return err
			}
		}
		return input.DispatchMouseEvent(input.MouseReleased, dstX, dstY).
			WithButton(input.Left).WithClickCount(1).Do(ctx)
	}))
}
