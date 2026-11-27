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
	Width       int64   `json:"w,omitempty"`
	Height      int64   `json:"h,omitempty"`
	Scale       float64 `json:"dpr,omitempty"`
	Mobile      bool    `json:"mobile,omitempty"`
	UserAgent   string  `json:"ua,omitempty"`
	ColorScheme string  `json:"scheme,omitempty"`
}

func (e emulationSpec) apply() chromedp.Action {
	var tasks chromedp.Tasks
	if e.Width > 0 {
		metrics := emulation.SetDeviceMetricsOverride(e.Width, e.Height, e.Scale, e.Mobile).
			WithScreenWidth(e.Width).WithScreenHeight(e.Height)
		if e.Mobile {
			orientation := &emulation.ScreenOrientation{Type: emulation.OrientationTypePortraitPrimary}
			if e.Width > e.Height {
				orientation = &emulation.ScreenOrientation{Type: emulation.OrientationTypeLandscapePrimary, Angle: 90}
			}
			metrics = metrics.WithScreenOrientation(orientation)
		}
		tasks = append(tasks, metrics)
	}
	if e.Mobile {
		tasks = append(tasks, emulation.SetTouchEmulationEnabled(true).WithMaxTouchPoints(5))
	}
	if e.UserAgent != "" {
		tasks = append(tasks, emulation.SetUserAgentOverride(e.UserAgent))
	}
	if e.ColorScheme != "" {
		tasks = append(tasks, emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{
			{Name: "prefers-color-scheme", Value: e.ColorScheme},
		}))
	}
	return tasks
}

func (e emulationSpec) clear() chromedp.Action {
	var tasks chromedp.Tasks
	if e.Width > 0 {
		tasks = append(tasks, emulation.ClearDeviceMetricsOverride())
	}
	if e.Mobile {
		tasks = append(tasks, emulation.SetTouchEmulationEnabled(false))
	}
	if e.UserAgent != "" {
		tasks = append(tasks, emulation.SetUserAgentOverride(""))
	}
	if e.ColorScheme != "" {
		tasks = append(tasks, emulation.SetEmulatedMedia())
	}
	return tasks
}

func (e emulationSpec) String() string {
	var parts []string
	if e.Width > 0 {
		s := fmt.Sprintf("%dx%d", e.Width, e.Height)
		if e.Scale > 0 {
			s += fmt.Sprintf(" @%gx", e.Scale)
		}
		parts = append(parts, s)
	}
	if e.Mobile {
		parts = append(parts, "mobile with touch")
	}
	if e.UserAgent != "" {
		parts = append(parts, "custom user agent")
	}
	if e.ColorScheme != "" {
		parts = append(parts, e.ColorScheme+" color scheme")
	}
	return strings.Join(parts, ", ")
}

func Resize(ctx context.Context, variant string, port int, args []string) {
	fs := flag.NewFlagSet("resize", flag.ExitOnError)
	reset := fs.Bool("reset", false, "clear the emulation and restore the window's own size")
	scale := fs.Float64("dpr", 0, "device pixel ratio (default: the screen's own)")
	mobile := fs.Bool("mobile", false, "emulate a phone: mobile viewport and touch input, mouse clicks become taps")
	userAgent := fs.String("user-agent", "", "user agent string (takes effect for requests after the next reload)")
	scheme := fs.String("color-scheme", "", "emulate prefers-color-scheme: dark or light")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools resize [--dpr <n>] [--mobile] [--user-agent <ua>] [--color-scheme dark|light] [<width> <height>]")
		fmt.Fprintln(os.Stderr, "       browser-tools resize --reset")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "A new size replaces the viewport; user agent and color scheme stay until changed or reset.")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	var width, height int64
	if !*reset {
		switch fs.NArg() {
		case 0:
			if *userAgent == "" && *scheme == "" {
				fs.Usage()
				os.Exit(1)
			}
		case 2:
			var err error
			if width, err = strconv.ParseInt(fs.Arg(0), 10, 64); err != nil || width <= 0 {
				fail("invalid width %q", fs.Arg(0))
			}
			if height, err = strconv.ParseInt(fs.Arg(1), 10, 64); err != nil || height <= 0 {
				fail("invalid height %q", fs.Arg(1))
			}
		default:
			fs.Usage()
			os.Exit(1)
		}
		if *scale < 0 {
			fail("invalid --dpr %g", *scale)
		}
		if *scheme != "" && *scheme != "dark" && *scheme != "light" {
			fail("invalid --color-scheme %q (use dark or light)", *scheme)
		}
		if width == 0 && (*mobile || *scale > 0) {
			fail("--mobile and --dpr need a width and height")
		}
	}

	tabCtx := MustTab(MustSession(ctx, variant, port))
	id := chromedp.FromContext(tabCtx).Target.TargetID

	var spec emulationSpec
	if prev := browser.CurrentEmulation(port, id); prev != "" {
		_ = json.Unmarshal([]byte(prev), &spec)
	}
	if width > 0 {
		spec.Width, spec.Height, spec.Scale, spec.Mobile = width, height, *scale, *mobile
	}
	if *userAgent != "" {
		spec.UserAgent = *userAgent
	}
	if *scheme != "" {
		spec.ColorScheme = *scheme
	}

	stopped := browser.StopEmulation(port, id)
	if *reset {
		if stopped {
			fmt.Printf("%s✓%s emulation cleared\n", ansiGreen, ansiReset)
		} else {
			fmt.Printf("%s✓%s nothing was emulated\n", ansiGreen, ansiReset)
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
