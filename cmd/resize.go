package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"browser-tools/browser"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	flag "github.com/spf13/pflag"
)

type emulationSpec struct {
	Width     int64   `json:"w"`
	Height    int64   `json:"h"`
	Scale     float64 `json:"dpr,omitempty"`
	Mobile    bool    `json:"mobile,omitempty"`
	UserAgent string  `json:"ua,omitempty"`
}

func (e emulationSpec) apply() chromedp.Action {
	metrics := emulation.SetDeviceMetricsOverride(e.Width, e.Height, e.Scale, e.Mobile).
		WithScreenWidth(e.Width).WithScreenHeight(e.Height)
	if e.Mobile {
		orientation := &emulation.ScreenOrientation{Type: emulation.OrientationTypePortraitPrimary}
		if e.Width > e.Height {
			orientation = &emulation.ScreenOrientation{Type: emulation.OrientationTypeLandscapePrimary, Angle: 90}
		}
		metrics = metrics.WithScreenOrientation(orientation)
	}
	tasks := chromedp.Tasks{metrics}
	if e.Mobile {
		tasks = append(tasks, emulation.SetTouchEmulationEnabled(true).WithMaxTouchPoints(5))
	}
	if e.UserAgent != "" {
		tasks = append(tasks, emulation.SetUserAgentOverride(e.UserAgent))
	}
	return tasks
}

func (e emulationSpec) clear() chromedp.Action {
	tasks := chromedp.Tasks{emulation.ClearDeviceMetricsOverride()}
	if e.Mobile {
		tasks = append(tasks, emulation.SetTouchEmulationEnabled(false))
	}
	if e.UserAgent != "" {
		tasks = append(tasks, emulation.SetUserAgentOverride(""))
	}
	return tasks
}

func (e emulationSpec) String() string {
	s := fmt.Sprintf("%dx%d", e.Width, e.Height)
	if e.Scale > 0 {
		s += fmt.Sprintf(" @%gx", e.Scale)
	}
	if e.Mobile {
		s += ", mobile with touch"
	}
	if e.UserAgent != "" {
		s += ", custom user agent"
	}
	return s
}

func Resize(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("resize", flag.ExitOnError)
	reset := fs.Bool("reset", false, "clear the emulation and restore the window's own size")
	scale := fs.Float64("dpr", 0, "device pixel ratio (default: the screen's own)")
	mobile := fs.Bool("mobile", false, "emulate a phone: mobile viewport and touch input, mouse clicks become taps")
	userAgent := fs.String("user-agent", "", "user agent string (takes effect for requests after the next reload)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools resize [--dpr <n>] [--mobile] [--user-agent <ua>] <width> <height>")
		fmt.Fprintln(os.Stderr, "       browser-tools resize --reset")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	var spec emulationSpec
	if !*reset {
		if fs.NArg() < 2 {
			fs.Usage()
			os.Exit(1)
		}
		width, err := strconv.ParseInt(fs.Arg(0), 10, 64)
		if err != nil || width <= 0 {
			fail("invalid width %q", fs.Arg(0))
		}
		height, err := strconv.ParseInt(fs.Arg(1), 10, 64)
		if err != nil || height <= 0 {
			fail("invalid height %q", fs.Arg(1))
		}
		if *scale < 0 {
			fail("invalid --dpr %g", *scale)
		}
		spec = emulationSpec{Width: width, Height: height, Scale: *scale, Mobile: *mobile, UserAgent: *userAgent}
	}

	tabCtx := MustTab(MustSession(ctx, variant, port))
	id := chromedp.FromContext(tabCtx).Target.TargetID

	stopped := browser.StopEmulation(port, id)
	if *reset {
		if stopped {
			fmt.Printf("%s✓%s reset viewport to default\n", ansiGreen, ansiReset)
		} else {
			fmt.Printf("%s✓%s viewport was not emulated\n", ansiGreen, ansiReset)
		}
		return
	}

	raw, _ := json.Marshal(spec)
	if err := browser.StartEmulation(variant, port, id, string(raw)); err != nil {
		fail("%v", err)
	}
	fmt.Printf("%s✓%s emulating %s until `resize --reset` or the tab closes\n", ansiGreen, ansiReset, spec)
}

// HoldEmulation applies an emulation to a tab and keeps the CDP session open,
// because Chrome clears the overrides as soon as it detaches. It answers "ok"
// or an error on stdout, then runs until its PID file is removed or the tab
// is gone.
func HoldEmulation(ctx context.Context, variant string, port int, args []string) {
	if len(args) != 2 {
		fmt.Println("error: usage: emulation-holder <target-id> <spec>")
		os.Exit(1)
	}
	id := target.ID(args[0])
	var spec emulationSpec
	if err := json.Unmarshal([]byte(args[1]), &spec); err != nil {
		fmt.Printf("error: invalid emulation spec: %v\n", err)
		os.Exit(1)
	}

	s, err := browser.Open(ctx, variant, port)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	tabCtx := s.Tab(id)
	if err := RunBounded(tabCtx, spec.apply()); err != nil {
		fmt.Printf("error: %v\n", strings.TrimSpace(err.Error()))
		os.Exit(1)
	}
	pidFile := browser.EmulationPIDFile(port, id)
	if err := browser.WritePID(pidFile); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("ok")
	_ = os.Stdout.Close()

	tick := time.NewTicker(100 * time.Millisecond)
	lastCheck := time.Now()
	for range tick.C {
		if browser.ReadPID(pidFile) != os.Getpid() {
			_ = RunBounded(tabCtx, spec.clear())
			os.Exit(0)
		}
		if time.Since(lastCheck) < 2*time.Second {
			continue
		}
		lastCheck = time.Now()
		if !tabExists(s, id) {
			_ = os.Remove(pidFile)
			os.Exit(0)
		}
	}
}

// tabExists reports whether the browser still answers and lists the tab.
func tabExists(s *browser.Session, id target.ID) bool {
	found := make(chan bool, 1)
	go func() {
		pages, err := s.Targets()
		found <- err == nil && slices.ContainsFunc(pages, func(t *target.Info) bool { return t.TargetID == id })
	}()
	select {
	case ok := <-found:
		return ok
	case <-time.After(tabPatience):
		return false
	}
}
