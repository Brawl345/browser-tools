package e2e

import (
	"bytes"
	"errors"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

const defaultTimeout = 5 * time.Second

// The watchdog fires 500ms after --timeout; process start and exit take the rest.
const overrun = 1500 * time.Millisecond

type result struct {
	code   int
	stdout string
	stderr string
	took   time.Duration
}

func (r result) output() string { return r.stdout + r.stderr }

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func needBrowser(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("needs a browser")
	}
}

func run(t *testing.T, args ...string) result {
	t.Helper()
	timeout := defaultTimeout
	full := []string{"--port", port}
	if i := slices.IndexFunc(args, func(a string) bool { return a == "--timeout" || strings.HasPrefix(a, "--timeout=") }); i >= 0 {
		v := strings.TrimPrefix(args[i], "--timeout=")
		if args[i] == "--timeout" && i+1 < len(args) {
			v = args[i+1]
		}
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	} else {
		full = append(full, "--timeout", timeout.String())
	}
	full = append(full, args...)

	var stdout, stderr bytes.Buffer
	c := exec.Command(binary, full...)
	c.Stdout, c.Stderr = &stdout, &stderr
	start := time.Now()
	err := c.Run()
	r := result{
		stdout: ansi.ReplaceAllString(stdout.String(), ""),
		stderr: ansi.ReplaceAllString(stderr.String(), ""),
		took:   time.Since(start),
	}
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		r.code = exit.ExitCode()
	case err != nil:
		t.Fatalf("running %q: %v", args, err)
	}
	if r.took > timeout+overrun {
		t.Errorf("%q took %s with --timeout %s", args, r.took.Round(time.Millisecond), timeout)
	}
	return r
}

func ok(t *testing.T, args ...string) result {
	t.Helper()
	r := run(t, args...)
	if r.code != 0 {
		t.Fatalf("%q failed with exit %d:\n%s", args, r.code, r.output())
	}
	return r
}

func fails(t *testing.T, args ...string) result {
	t.Helper()
	r := run(t, args...)
	if r.code == 0 {
		t.Fatalf("%q succeeded, want an error:\n%s", args, r.output())
	}
	return r
}

func contains(t *testing.T, r result, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(r.output(), w) {
			t.Errorf("output lacks %q:\n%s", w, r.output())
		}
	}
}

func lacks(t *testing.T, r result, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(r.output(), u) {
			t.Errorf("output has %q:\n%s", u, r.output())
		}
	}
}

func lines(s string) []string {
	var out []string
	for l := range strings.SplitSeq(strings.TrimSpace(s), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

var tabLine = regexp.MustCompile(`^(?:▶\s+)?(\d+)\s`)

func tabCount(t *testing.T) int {
	t.Helper()
	n := 0
	for _, l := range lines(ok(t, "tab").stdout) {
		if tabLine.MatchString(l) {
			n++
		}
	}
	return n
}

func reset(t *testing.T) {
	t.Helper()
	needBrowser(t)
	for n := tabCount(t); n > 1; n-- {
		ok(t, "tab", "--close", strconv.Itoa(n))
	}
	ok(t, "navigate", pageURL("/blank.html"))
	ok(t, "clear", "--all", "--all-origins")
	ok(t, "resize", "--reset")
	ok(t, "navigate", "about:blank")
}

func open(t *testing.T, page string) {
	t.Helper()
	reset(t)
	ok(t, "navigate", pageURL(page))
}
