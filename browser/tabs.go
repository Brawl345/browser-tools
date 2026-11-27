package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	cdp "github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// probeTimeout caps a round of visibility probes, so a tab stuck in a long
// script cannot stall tab discovery.
const probeTimeout = 700 * time.Millisecond

// historyLimit is how many earlier current tabs are remembered, so closing a
// popup returns to the page that opened it.
const historyLimit = 20

// tabState is what one command hands to the next about a browser's tabs. Every
// command is its own process, and Chrome cannot say which tab is selected
// while its window is in the background or covered.
type tabState struct {
	Seen    []TargetID `json:"seen"`
	History []TargetID `json:"history"`
}

func tabStateFile(port int) string {
	return filepath.Join(cacheBase(), "tabs", fmt.Sprintf("%d.json", port))
}

func loadTabState(port int) tabState {
	var st tabState
	if data, err := os.ReadFile(tabStateFile(port)); err == nil {
		_ = json.Unmarshal(data, &st)
	}
	return st
}

func (st tabState) save(port int) {
	path := tabStateFile(port)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, _ := json.Marshal(st)
	tmp := fmt.Sprintf("%s.%d", path, os.Getpid())
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}

// Targets returns every page tab in the order they were opened.
func (s *Session) Targets() ([]*target.Info, error) {
	c := chromedp.FromContext(s.brCtx)
	all, err := target.GetTargets().Do(cdp.WithExecutor(s.brCtx, c.Browser))
	if err != nil {
		return nil, err
	}
	var pages []*target.Info
	for _, t := range all {
		if t.Type == "page" && t.Subtype == "" {
			pages = append(pages, t)
		}
	}
	// Chrome mostly lists the newest target first, but not reliably, so tabs
	// already seen keep the place they were first seen at.
	slices.Reverse(pages)
	rank := map[TargetID]int{}
	for i, id := range loadTabState(s.port).Seen {
		rank[id] = i
	}
	slices.SortStableFunc(pages, func(a, b *target.Info) int {
		ra, okA := rank[a.TargetID]
		rb, okB := rank[b.TargetID]
		switch {
		case okA && okB:
			return ra - rb
		case okA:
			return -1
		case okB:
			return 1
		}
		return 0
	})
	return pages, nil
}

// Tabs returns every page tab as display info, in the order they were opened.
func (s *Session) Tabs() ([]TabInfo, error) {
	pages, err := s.Targets()
	if err != nil {
		return nil, err
	}
	tabs := make([]TabInfo, 0, len(pages))
	for _, t := range pages {
		tabs = append(tabs, TabInfo{ID: string(t.TargetID), Title: t.Title, URL: t.URL})
	}
	return tabs, nil
}

// MakeCurrent records a tab as the one commands act on. Every other open tab
// counts as seen from then on, so none of them takes over later.
func (s *Session) MakeCurrent(id TargetID) {
	pages, _ := s.Targets()
	st := loadTabState(s.port)
	for _, p := range pages {
		if !slices.Contains(st.Seen, p.TargetID) {
			st.Seen = append(st.Seen, p.TargetID)
		}
	}
	if !slices.Contains(st.Seen, id) {
		st.Seen = append(st.Seen, id)
	}
	st.History = append(slices.DeleteFunc(st.History, func(h TargetID) bool { return h == id }), id)
	if len(st.History) > historyLimit {
		st.History = st.History[len(st.History)-historyLimit:]
	}
	st.save(s.port)
}

// ActiveTarget returns the current tab: the one a command last opened,
// navigated or activated. A tab that opened since in the foreground, as a popup
// or from a page link takes over, so a dialog or OAuth window is picked up
// without a detour through `tab`. When the current tab is gone, the one before
// it comes back.
func (s *Session) ActiveTarget() (*target.Info, error) {
	pages, err := s.Targets()
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, nil
	}

	st := loadTabState(s.port)
	open := map[TargetID]*target.Info{}
	for _, p := range pages {
		open[p.TargetID] = p
	}
	known := slices.ContainsFunc(st.Seen, func(id TargetID) bool { return open[id] != nil })

	var pick *target.Info
	if known {
		var fresh []*target.Info
		for _, p := range pages {
			if !slices.Contains(st.Seen, p.TargetID) {
				fresh = append(fresh, p)
			}
		}
		if len(fresh) > 0 {
			pick = s.takeOver(fresh, pages, st.Seen)
		}
		for i := len(st.History) - 1; pick == nil && i >= 0; i-- {
			pick = open[st.History[i]]
			// Chrome selects a neighbour of a closed tab, and a tab left in the
			// background does not take input, so the one before comes forward.
			if pick != nil && i < len(st.History)-1 {
				_ = s.activate(pick.TargetID)
			}
		}
	}
	if pick == nil {
		pick = s.mostVisible(pages)
	}

	seen := make([]TargetID, len(pages))
	for i, p := range pages {
		seen[i] = p.TargetID
	}
	history := slices.DeleteFunc(slices.Clone(st.History), func(id TargetID) bool { return open[id] == nil || id == pick.TargetID })
	history = append(history, pick.TargetID)
	if len(history) > historyLimit {
		history = history[len(history)-historyLimit:]
	}
	next := tabState{Seen: seen, History: history}
	if !slices.Equal(next.Seen, st.Seen) || !slices.Equal(next.History, st.History) {
		next.save(s.port)
	}
	return pick, nil
}

// takeOver picks the newest of the freshly opened tabs that the user would be
// looking at: one selected in its window, one a page opened, or one in a window
// of its own. Tabs opened in the background stay in the background.
func (s *Session) takeOver(fresh, pages []*target.Info, seen []TargetID) *target.Info {
	visible := s.visible(fresh)
	for i := len(fresh) - 1; i >= 0; i-- {
		if visible[i] || fresh[i].OpenerID != "" {
			return fresh[i]
		}
	}

	// A covered window reports every tab as hidden, so a popup window shows
	// itself only through its window.
	oldWindows := map[cdpbrowser.WindowID]bool{}
	for _, p := range pages {
		if slices.Contains(seen, p.TargetID) {
			oldWindows[s.window(p.TargetID)] = true
		}
	}
	for i := len(fresh) - 1; i >= 0; i-- {
		if w := s.window(fresh[i].TargetID); w != 0 && !oldWindows[w] {
			return fresh[i]
		}
	}
	return nil
}

func (s *Session) window(id TargetID) cdpbrowser.WindowID {
	var w cdpbrowser.WindowID
	_ = s.Do(func(ctx context.Context) error {
		var err error
		w, _, err = cdpbrowser.GetWindowForTarget().WithTargetID(id).Do(ctx)
		return err
	})
	return w
}

// mostVisible is the fallback without any history: the newest tab that is
// selected in its window, or the newest tab at all.
func (s *Session) mostVisible(pages []*target.Info) *target.Info {
	visible := s.visible(pages)
	for i := len(pages) - 1; i >= 0; i-- {
		if visible[i] {
			return pages[i]
		}
	}
	return pages[len(pages)-1]
}

// visible asks all tabs at once under one deadline whether they are selected
// in their window, so a tab busy with a long script costs no more than one that
// answers straight away.
func (s *Session) visible(pages []*target.Info) []bool {
	flags := make([]atomic.Bool, len(pages))
	var wg sync.WaitGroup
	for i, p := range pages {
		wg.Go(func() {
			var v bool
			if chromedp.Run(s.Tab(p.TargetID), chromedp.Evaluate(`document.visibilityState === "visible"`, &v)) == nil {
				flags[i].Store(v)
			}
		})
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(probeTimeout):
		// Probes are left running. Cancelling one would close its tab.
	}
	out := make([]bool, len(pages))
	for i := range flags {
		out[i] = flags[i].Load()
	}
	return out
}
