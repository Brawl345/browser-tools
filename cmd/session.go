package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"browser-tools/browser"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
)

// MustSession connects to the browser, starting it when it is not running.
func MustSession(ctx context.Context, variant string, port int) *browser.Session {
	s, err := browser.Open(ctx, variant, port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return s
}

// MustTab returns the tab the user is looking at.
func MustTab(s *browser.Session) context.Context {
	tabCtx, err := s.ActiveTab()
	if err != nil || tabCtx == nil {
		fmt.Fprintf(os.Stderr, "error: no usable browser tab: %v\n", err)
		os.Exit(1)
	}
	return tabCtx
}

// ErrTabStalled is returned when a tab stops answering, which is what happens
// when the page closes its own tab while a command is still working on it.
var ErrTabStalled = errors.New("the tab stopped answering (it may have been closed)")

// tabPatience bounds a single round trip to a tab. Contexts here carry no
// cancellation on purpose - cancelling one closes the user's tab - so the bound
// has to live outside the context.
const tabPatience = 3 * time.Second

// RunBounded runs actions against a tab and gives up when the tab goes away.
func RunBounded(tabCtx context.Context, actions ...chromedp.Action) error {
	errc := make(chan error, 1)
	go func() { errc <- chromedp.Run(tabCtx, actions...) }()
	select {
	case err := <-errc:
		return err
	case <-time.After(tabPatience):
		return ErrTabStalled
	}
}

// OnHit evaluates a JS function body with `el` bound to the element the last
// Locate call resolved, and decodes its return value into out.
func OnHit(tabCtx context.Context, body string, out any) error {
	script := fmt.Sprintf(`(function(el){ if (!el) return null; %s })(window.__btHit)`, body)
	var raw []byte
	if err := RunBounded(tabCtx, chromedp.Evaluate(script, &raw)); err != nil {
		return err
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// PointerAt dispatches real mouse events at a viewport position. On a page
// with touch emulation (`resize --mobile`) a left click is sent as a tap.
func PointerAt(tabCtx context.Context, x, y float64, button input.MouseButton, clicks int64) error {
	if clicks > 0 && button == input.Left {
		var touch bool
		if err := RunBounded(tabCtx, chromedp.Evaluate(`navigator.maxTouchPoints > 0`, &touch)); err != nil {
			return err
		}
		if touch {
			return TapAt(tabCtx, x, y, clicks)
		}
	}
	return RunBounded(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		if err := input.DispatchMouseEvent(input.MouseMoved, x, y).Do(ctx); err != nil {
			return err
		}
		if clicks == 0 {
			return nil
		}
		for i := int64(1); i <= clicks; i++ {
			if err := input.DispatchMouseEvent(input.MousePressed, x, y).
				WithButton(button).WithButtons(1).WithClickCount(i).Do(ctx); err != nil {
				return err
			}
			if err := input.DispatchMouseEvent(input.MouseReleased, x, y).
				WithButton(button).WithClickCount(i).Do(ctx); err != nil {
				return err
			}
		}
		return nil
	}))
}

// TapAt dispatches touch taps at a viewport position.
func TapAt(tabCtx context.Context, x, y float64, taps int64) error {
	return RunBounded(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		point := []*input.TouchPoint{{X: x, Y: y, RadiusX: 4, RadiusY: 4, Force: 1}}
		for range taps {
			if err := input.DispatchTouchEvent(input.TouchStart, point).Do(ctx); err != nil {
				return err
			}
			if err := input.DispatchTouchEvent(input.TouchEnd, []*input.TouchPoint{}).Do(ctx); err != nil {
				return err
			}
		}
		return nil
	}))
}

// Focus puts the keyboard focus on the element the last Locate call resolved.
func Focus(tabCtx context.Context) error {
	return OnHit(tabCtx, `el.focus({preventScroll: true}); return true;`, nil)
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(1)
}
