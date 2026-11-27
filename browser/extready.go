package browser

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	cdp "github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// extensionWait bounds how long a fresh launch waits for extension workers to
// start. extensionSettle follows, because a worker keeps initialising
// asynchronously after its top-level code ran: Tampermonkey needed about 0.6s
// before it caught a .user.js.
const (
	extensionWait   = 5 * time.Second
	extensionSettle = 1500 * time.Millisecond
)

// waitForExtensions returns once every installed extension with a background
// script has had time to set itself up. Until then an extension misses what
// the first command does: Tampermonkey lets a .user.js load as plain text.
func waitForExtensions(variant string, port int) {
	pending := backgroundExtensions(UserDataDir(variant))
	if len(pending) == 0 {
		return
	}
	brCtx, err := connect(context.Background(), port)
	if err != nil {
		return
	}
	c := chromedp.FromContext(brCtx)
	deadline := time.Now().Add(extensionWait)
	attached := map[string]context.Context{}
	for len(pending) > 0 && time.Now().Before(deadline) {
		all, err := target.GetTargets().Do(cdp.WithExecutor(brCtx, c.Browser))
		if err != nil {
			return
		}
		for _, t := range all {
			if t.Type != "service_worker" && t.Type != "background_page" {
				continue
			}
			id, ok := extensionID(t.URL)
			if !ok || !pending[id] {
				continue
			}
			if attached[id] == nil {
				attached[id] = AttachTab(brCtx, t.TargetID)
			}
			// A worker answers only once its top-level code has run.
			if evalWithin(attached[id], time.Until(deadline)) {
				delete(pending, id)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(extensionSettle)
}

func evalWithin(ctx context.Context, d time.Duration) bool {
	done := make(chan error, 1)
	go func() {
		var v int
		done <- chromedp.Run(ctx, chromedp.Evaluate(`1`, &v))
	}()
	select {
	case err := <-done:
		return err == nil
	case <-time.After(d):
		return false
	}
}

func extensionID(url string) (string, bool) {
	rest, ok := strings.CutPrefix(url, "chrome-extension://")
	if !ok {
		return "", false
	}
	id, _, _ := strings.Cut(rest, "/")
	return id, id != ""
}

// backgroundExtensions lists the enabled extensions of a profile that come
// with a background script. Built-in component extensions are left out.
func backgroundExtensions(userDataDir string) map[string]bool {
	out := map[string]bool{}
	for _, name := range []string{"Secure Preferences", "Preferences"} {
		data, err := os.ReadFile(filepath.Join(userDataDir, "Default", name))
		if err != nil {
			continue
		}
		var prefs struct {
			Extensions struct {
				Settings map[string]struct {
					Location       int             `json:"location"`
					DisableReasons json.RawMessage `json:"disable_reasons"`
					Manifest       struct {
						Background json.RawMessage `json:"background"`
					} `json:"manifest"`
				} `json:"settings"`
			} `json:"extensions"`
		}
		if json.Unmarshal(data, &prefs) != nil {
			continue
		}
		for id, s := range prefs.Extensions.Settings {
			const component, externalComponent = 5, 10
			if s.Location == component || s.Location == externalComponent {
				continue
			}
			if len(s.Manifest.Background) == 0 || !noReasons(s.DisableReasons) {
				continue
			}
			out[id] = true
		}
	}
	return out
}

func noReasons(raw json.RawMessage) bool {
	r := strings.TrimSpace(string(raw))
	return r == "" || r == "[]" || r == "0" || r == "null"
}
