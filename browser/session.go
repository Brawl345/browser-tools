package browser

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	cdp "github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// TargetID re-exports the cdproto target.ID type for use in cmd package.
type TargetID = target.ID

func IsRunning(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// Launch starts the browser; extra is appended to its command line.
func Launch(variant string, port int, extra ...string) error {
	cfg, ok := GetConfig(variant)
	if !ok {
		return fmt.Errorf("unknown browser variant: %s", variant)
	}
	if _, err := os.Stat(cfg.Executable); err != nil {
		return fmt.Errorf("%s not found at %s – is it installed?", cfg.ProcessName, cfg.Executable)
	}

	udd := UserDataDir(variant)
	if err := os.MkdirAll(udd, 0o755); err != nil {
		return fmt.Errorf("failed to create user data dir: %w", err)
	}

	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		fmt.Sprintf("--user-data-dir=%s", udd),
		"--no-first-run",
		"--no-default-browser-check",
	}
	if IsChromeForTesting(variant) {
		args = append(args, "--enable-unsafe-extension-debugging")
	}
	args = append(args, extra...)

	c := exec.Command(cfg.Executable, args...)
	c.Stdout = nil
	c.Stderr = nil
	detachProcess(c)

	fmt.Fprintf(os.Stderr, "Starting %s on port %d...\n", cfg.ProcessName, port)
	return KeepFront(launchRaiseWindow, func() error {
		if err := c.Start(); err != nil {
			return fmt.Errorf("failed to start browser: %w", err)
		}
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(500 * time.Millisecond)
			if IsRunning(port) {
				return nil
			}
		}
		return fmt.Errorf("browser did not become ready within 15 seconds")
	})
}

// launchRaiseWindow and activateRaiseWindow bound how long the browser may take
// to raise its window after a launch or a tab activation.
const (
	launchRaiseWindow   = 1500 * time.Millisecond
	activateRaiseWindow = 500 * time.Millisecond
)

// EnsureRunning launches the browser only when nothing listens on the port. A
// browser that holds the port but is slow to answer (macOS App Nap throttles a
// background window) is waited for instead, because a second launch would
// open a new window on the same profile.
func EnsureRunning(variant string, port int) error {
	if IsRunning(port) {
		return nil
	}
	if !portInUse(port) {
		return Launch(variant, port)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if IsRunning(port) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("something listens on port %d but does not answer as a browser", port)
}

func portInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func quietOpts() []chromedp.ContextOption {
	return []chromedp.ContextOption{
		chromedp.WithLogf(func(string, ...any) {}),
		chromedp.WithErrorf(func(string, ...any) {}),
	}
}

// AttachTab returns a context bound to an existing tab. The returned context has
// no cancel function on purpose - cancelling it makes chromedp close that tab.
// The parent must already carry the browser connection: a context that has to
// allocate one panics on its second Run once the first allocation failed.
func AttachTab(brCtx context.Context, id TargetID) context.Context {
	ctx, _ := chromedp.NewContext(brCtx, chromedp.WithTargetID(id))
	return ctx
}

// TabInfo holds display information about a browser tab.
type TabInfo struct {
	ID    string
	Title string
	URL   string
}

// Session is a connection to a running browser. It owns a browser-level
// websocket that answers target queries without opening a throwaway tab, and
// caches one attached context per tab so repeated lookups stay cheap.
type Session struct {
	brCtx context.Context
	port  int
	mu    sync.Mutex
	tabs  map[TargetID]context.Context
}

// Open starts the browser if needed and connects to it.
func Open(ctx context.Context, variant string, port int) (*Session, error) {
	if err := EnsureRunning(variant, port); err != nil {
		return nil, err
	}
	if DialogAnswer != "" {
		EnsureDialogKeeper(variant, port)
		ReportKeptDialogs(port)
	}
	// Cancellation is cut off here: chromedp closes the target of every context
	// that ends, and with a RemoteAllocator that includes tabs attached through
	// WithTargetID. A deadline reaching a tab context would close the user's tab.
	allocCtx, _ := chromedp.NewRemoteAllocator(context.WithoutCancel(ctx), fmt.Sprintf("http://127.0.0.1:%d", port))

	// A chromedp context normally creates its own tab on first use. Allocating
	// the browser by hand attaches to the websocket only, so target queries and
	// tab listings never disturb the user's window.
	brCtx, _ := chromedp.NewContext(allocCtx, quietOpts()...)
	c := chromedp.FromContext(brCtx)
	b, err := c.Allocator.Allocate(brCtx)
	if err != nil {
		return nil, fmt.Errorf("cannot talk to the browser on port %d: %w", port, err)
	}
	c.Browser = b

	return &Session{brCtx: brCtx, port: port, tabs: map[TargetID]context.Context{}}, nil
}

// Tab returns a cached context attached to the given tab.
func (s *Session) Tab(id TargetID) context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx, ok := s.tabs[id]; ok {
		return ctx
	}
	ctx := AttachTab(s.brCtx, id)
	if DialogAnswer != "" {
		answerDialogs(ctx)
	}
	s.tabs[id] = ctx
	return ctx
}

// Do runs a browser-level CDP call. The context it hands over already carries
// the browser executor, so domains that live outside a tab (Target, Extensions,
// Browser) work without opening one.
func (s *Session) Do(fn func(ctx context.Context) error) error {
	c := chromedp.FromContext(s.brCtx)
	return fn(cdp.WithExecutor(s.brCtx, c.Browser))
}

// ActiveTab returns a context for the current tab, opening one when the
// browser has no page tab at all.
func (s *Session) ActiveTab() (context.Context, error) {
	t, err := s.ActiveTarget()
	if err != nil {
		return nil, err
	}
	if t == nil {
		return s.NewTab()
	}
	return s.Tab(t.TargetID), nil
}

// NewTab opens a blank tab and attaches to it.
func (s *Session) NewTab() (context.Context, error) {
	var id TargetID
	err := s.Do(func(ctx context.Context) error {
		var err error
		id, err = target.CreateTarget("about:blank").Do(ctx)
		if err != nil && strings.Contains(err.Error(), "no browser is open") {
			// On macOS Chrome keeps running without a window once its last tab
			// is closed, and only a new window can host a tab then.
			err = KeepFront(launchRaiseWindow, func() error {
				id, err = target.CreateTarget("about:blank").WithNewWindow(true).Do(ctx)
				return err
			})
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	s.MakeCurrent(id)
	return s.Tab(id), nil
}

// ActivateTabByID selects a tab by its target ID, leaving the frontmost app in
// front.
func (s *Session) ActivateTabByID(id string) error {
	err := s.activate(TargetID(id))
	if err == nil {
		s.MakeCurrent(TargetID(id))
	}
	return err
}

func (s *Session) activate(id TargetID) error {
	c := chromedp.FromContext(s.brCtx)
	return KeepFront(activateRaiseWindow, func() error {
		return target.ActivateTarget(id).Do(cdp.WithExecutor(s.brCtx, c.Browser))
	})
}

// CloseTabByID closes a tab by its target ID.
func (s *Session) CloseTabByID(id string) error {
	c := chromedp.FromContext(s.brCtx)
	return target.CloseTarget(target.ID(id)).Do(cdp.WithExecutor(s.brCtx, c.Browser))
}

// StartNavigate fires a navigation without waiting for the page to fully load.
func StartNavigate(url string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		_, _, _, _, err := page.Navigate(url).Do(ctx)
		return err
	}
}

// ActivateCurrentTab brings the current tab and its window into focus.
func ActivateCurrentTab() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		return target.ActivateTarget(c.Target.TargetID).Do(cdp.WithExecutor(ctx, c.Browser))
	}
}

// SelectCurrentTab selects the current tab in its window, leaving the frontmost
// app in front.
func SelectCurrentTab() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		return KeepFront(activateRaiseWindow, func() error { return ActivateCurrentTab().Do(ctx) })
	}
}
