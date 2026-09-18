package cmd

import (
	"context"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Quiet windows used after an action and after a navigation. They are short on
// purpose: the point is to absorb the request a click kicks off, not to wait
// for every tracker on the page.
const (
	settleQuiet   = 250 * time.Millisecond
	settleAction  = 1500 * time.Millisecond
	settleLoad    = 1500 * time.Millisecond
	loadEventWait = 12 * time.Second
)

// watcher counts in-flight requests and remembers when the page last did
// anything.
type watcher struct {
	mu        sync.Mutex
	inflight  int
	last      time.Time
	loaded    bool
	navStated bool
	start     time.Time
}

func watch(tabCtx context.Context) *watcher {
	w := &watcher{last: time.Now(), start: time.Now()}
	chromedp.ListenTarget(tabCtx, func(ev any) {
		w.mu.Lock()
		defer w.mu.Unlock()
		switch ev.(type) {
		case *network.EventRequestWillBeSent:
			w.inflight++
			w.last = time.Now()
		case *network.EventLoadingFinished, *network.EventLoadingFailed:
			if w.inflight > 0 {
				w.inflight--
			}
			w.last = time.Now()
		case *page.EventFrameStartedLoading, *page.EventFrameNavigated:
			w.loaded = false
			w.navStated = true
			w.last = time.Now()
		case *page.EventLoadEventFired, *page.EventDomContentEventFired:
			w.loaded = true
			w.last = time.Now()
		}
	})
	_ = RunBounded(tabCtx, network.Enable(), page.Enable())
	return w
}

func (w *watcher) idle(quiet time.Duration) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.inflight == 0 && time.Since(w.last) > quiet
}

// done reports that the page has nothing left to do: the load event arrived, or
// the navigation turned out to be a same-document jump that never fires one.
func (w *watcher) done() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.loaded || (!w.navStated && time.Since(w.start) > 600*time.Millisecond)
}

// SettleAfter starts listening and hands back the wait to run once the action
// is done. Listening has to begin beforehand, otherwise a navigation the action
// kicks off is already under way when the wait starts and looks like silence.
func SettleAfter(tabCtx context.Context) func() {
	w := watch(tabCtx)
	return func() { settleWith(w, settleAction) }
}

func settleWith(w *watcher, max time.Duration) {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		if w.idle(settleQuiet) {
			return
		}
		time.Sleep(40 * time.Millisecond)
	}
}

// WaitLoaded blocks until the page fires its load event and then goes quiet.
// It reports whether the load event arrived in time; a page that keeps loading
// is not an error, the caller just says so.
func WaitLoaded(ctx context.Context, tabCtx context.Context, w *watcher) bool {
	deadline := time.Now().Add(loadEventWait)
	if d, ok := ctx.Deadline(); ok {
		deadline = d.Add(-700 * time.Millisecond)
	}
	for time.Now().Before(deadline) {
		if w.done() {
			// Pages that poll in the background never fall silent, so the quiet
			// period after the load event is capped rather than required.
			settleWith(w, settleLoad)
			return true
		}
		time.Sleep(40 * time.Millisecond)
	}
	return false
}
