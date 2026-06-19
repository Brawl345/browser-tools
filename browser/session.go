package browser

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
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

func Launch(variant string, port int) error {
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

	c := exec.Command(cfg.Executable, args...)
	c.Stdout = nil
	c.Stderr = nil
	detachProcess(c)

	fmt.Fprintf(os.Stderr, "Starting %s on port %d...\n", cfg.ProcessName, port)
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
}

func EnsureRunning(variant string, port int) error {
	if IsRunning(port) {
		return nil
	}
	return Launch(variant, port)
}

// Connect returns a chromedp allocator context connected to a browser on the given port.
func Connect(ctx context.Context, port int) (context.Context, context.CancelFunc) {
	return chromedp.NewRemoteAllocator(ctx, fmt.Sprintf("http://127.0.0.1:%d", port))
}

// NewTab opens a new browser tab within an allocator context.
func NewTab(allocCtx context.Context) (context.Context, context.CancelFunc) {
	return chromedp.NewContext(
		allocCtx,
		chromedp.WithLogf(func(string, ...any) {}),
		chromedp.WithErrorf(func(string, ...any) {}),
	)
}

// TabInfo holds display information about a browser tab.
type TabInfo struct {
	ID    string
	Title string
	URL   string
}

// GetPageTabs returns all open page tabs from within an action context, excluding the given target ID.
// Results are sorted by target ID to ensure a stable, deterministic order across calls.
func GetPageTabs(ctx context.Context, excludeID string) ([]TabInfo, error) {
	c := chromedp.FromContext(ctx)
	all, err := target.GetTargets().Do(cdp.WithExecutor(ctx, c.Browser))
	if err != nil {
		return nil, err
	}
	var tabs []TabInfo
	for _, t := range all {
		if t.Type == "page" && string(t.TargetID) != excludeID {
			tabs = append(tabs, TabInfo{ID: string(t.TargetID), Title: t.Title, URL: t.URL})
		}
	}
	sort.Slice(tabs, func(i, j int) bool { return tabs[i].ID < tabs[j].ID })
	return tabs, nil
}

// ActivateTabByID activates a tab by its target ID from within an action context.
func ActivateTabByID(id string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		return target.ActivateTarget(target.ID(id)).Do(cdp.WithExecutor(ctx, c.Browser))
	}
}

// CloseTabByID closes a tab by its target ID from within an action context.
func CloseTabByID(id string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		return target.CloseTarget(target.ID(id)).Do(cdp.WithExecutor(ctx, c.Browser))
	}
}

// visibleTabID checks visibilityState on each tab in pageTabs and returns
// the TargetID of the first visible one. No bootstrap tab is created.
func visibleTabID(allocCtx context.Context, pageTabs []*target.Info) string {
	for _, t := range pageTabs {
		tabCtx, _ := chromedp.NewContext(
			allocCtx,
			chromedp.WithTargetID(t.TargetID),
			chromedp.WithLogf(func(string, ...any) {}),
			chromedp.WithErrorf(func(string, ...any) {}),
		)
		var state string
		if err := chromedp.Run(tabCtx, chromedp.Evaluate(`document.visibilityState`, &state)); err == nil && state == "visible" {
			return string(t.TargetID)
		}
	}
	return ""
}

// ActiveTabID returns the TargetID of the currently visible page tab.
// Creates one bootstrap tab to enumerate targets, then uses visibleTabID.
func ActiveTabID(allocCtx context.Context, exclude string) string {
	bootstrapCtx, bootstrapCancel := NewTab(allocCtx)
	defer bootstrapCancel()

	var allTargets []*target.Info
	if err := chromedp.Run(bootstrapCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		var err error
		allTargets, err = target.GetTargets().Do(cdp.WithExecutor(ctx, c.Browser))
		return err
	})); err != nil {
		return ""
	}

	bootstrapID := string(chromedp.FromContext(bootstrapCtx).Target.TargetID)
	var pageTabs []*target.Info
	for _, t := range allTargets {
		if t.Type == "page" && t.Subtype == "" &&
			string(t.TargetID) != bootstrapID && string(t.TargetID) != exclude {
			pageTabs = append(pageTabs, t)
		}
	}
	return visibleTabID(allocCtx, pageTabs)
}

// ExistingOrNewTab attaches to the currently visible page tab.
// Creates exactly one bootstrap tab to enumerate targets, then reuses that
// target list for the visibilityState check - no second bootstrap is opened.
func ExistingOrNewTab(allocCtx context.Context) (context.Context, context.CancelFunc) {
	bootstrapCtx, bootstrapCancel := NewTab(allocCtx)

	var allTargets []*target.Info
	if err := chromedp.Run(bootstrapCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		var err error
		allTargets, err = target.GetTargets().Do(cdp.WithExecutor(ctx, c.Browser))
		return err
	})); err != nil {
		return bootstrapCtx, bootstrapCancel
	}

	bootstrapID := chromedp.FromContext(bootstrapCtx).Target.TargetID
	bootstrapCancel()

	var pageTabs []*target.Info
	for _, t := range allTargets {
		if t.Type == "page" && t.TargetID != bootstrapID && t.Subtype == "" {
			pageTabs = append(pageTabs, t)
		}
	}

	if len(pageTabs) == 0 {
		return NewTab(allocCtx)
	}

	newTabCtx := func(id target.ID) (context.Context, context.CancelFunc) {
		return chromedp.NewContext(
			allocCtx,
			chromedp.WithTargetID(id),
			chromedp.WithLogf(func(string, ...any) {}),
			chromedp.WithErrorf(func(string, ...any) {}),
		)
	}

	if activeID := visibleTabID(allocCtx, pageTabs); activeID != "" {
		return newTabCtx(target.ID(activeID))
	}

	// Fallback: first page tab.
	return newTabCtx(pageTabs[0].TargetID)
}

// StartNavigate fires a navigation without waiting for the page to fully load.
func StartNavigate(url string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		_, _, _, _, err := page.Navigate(url).Do(ctx)
		return err
	}
}

// CloseCurrentTab closes the tab from within its own context.
func CloseCurrentTab() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		return target.CloseTarget(c.Target.TargetID).Do(cdp.WithExecutor(ctx, c.Browser))
	}
}

// ActivateCurrentTab brings the current tab into focus.
func ActivateCurrentTab() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		c := chromedp.FromContext(ctx)
		return target.ActivateTarget(c.Target.TargetID).Do(cdp.WithExecutor(ctx, c.Browser))
	}
}
