package e2e

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func eventually(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s did not happen within %s", what, d)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return ansi.ReplaceAllString(b.buf.String(), "")
}

type streamer struct {
	args           []string
	c              *exec.Cmd
	stdout, stderr syncBuffer
	done           chan error
}

// stream starts a command that runs until SIGINT and returns once it printed ready.
func stream(t *testing.T, ready string, args ...string) *streamer {
	t.Helper()
	s := &streamer{args: args, done: make(chan error, 1)}
	s.c = exec.Command(binary, append([]string{"--port", port}, args...)...)
	s.c.Stdout, s.c.Stderr = &s.stdout, &s.stderr
	if err := s.c.Start(); err != nil {
		t.Fatalf("starting %q: %v", args, err)
	}
	go func() { s.done <- s.c.Wait() }()
	t.Cleanup(func() { _ = s.c.Process.Kill() })

	deadline := time.Now().Add(defaultTimeout)
	for !strings.Contains(s.stderr.String(), ready) {
		select {
		case err := <-s.done:
			t.Fatalf("%q exited before it was ready (%v):\n%s%s", args, err, s.stdout.String(), s.stderr.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("%q did not print %q within %s:\n%s", args, ready, defaultTimeout, s.stderr.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	return s
}

func (s *streamer) stop(t *testing.T) result {
	t.Helper()
	if err := s.c.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("interrupting %q: %v", s.args, err)
	}
	select {
	case err := <-s.done:
		r := result{stdout: s.stdout.String(), stderr: s.stderr.String()}
		if err != nil {
			t.Errorf("%q exited with %v after SIGINT:\n%s", s.args, err, r.output())
		}
		return r
	case <-time.After(3 * time.Second):
		t.Fatalf("%q still runs 3s after SIGINT", s.args)
		return result{}
	}
}

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("%s is no PNG: %v", path, err)
	}
	return img
}

func jsFloat(t *testing.T, script string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(js(t, script), 64)
	if err != nil {
		t.Fatalf("%s: %v", script, err)
	}
	return v
}

func wantSize(t *testing.T, what string, img image.Image, w, h float64) {
	t.Helper()
	b := img.Bounds()
	if dx, dy := float64(b.Dx())-w, float64(b.Dy())-h; dx < -1 || dx > 1 || dy < -1 || dy > 1 {
		t.Errorf("%s is %dx%d, want %.0fx%.0f", what, b.Dx(), b.Dy(), w, h)
	}
}

// Screenshots carry the display's colour profile (pure red comes out as
// rgb(234, 51, 35) on a P3 screen), so only the dominant channel is compared.
func wantColor(t *testing.T, what string, img image.Image, x, y float64, want string) {
	t.Helper()
	r, g, b, _ := img.At(int(x), int(y)).RGBA()
	got := "mixed"
	switch {
	case r > 2*g && r > 2*b:
		got = "red"
	case b > 2*r && b > 2*g:
		got = "blue"
	}
	if got != want {
		t.Errorf("%s at %.0f,%.0f is rgb(%d, %d, %d), want %s", what, x, y, r>>8, g>>8, b>>8, want)
	}
}

const (
	red  = "red"
	blue = "blue"
)

func TestScreenshot(t *testing.T) {
	open(t, "/capture.html")
	dpr := jsFloat(t, "devicePixelRatio")
	dir := t.TempDir()

	t.Run("viewport waits for a late image", func(t *testing.T) {
		js(t, `document.getElementById("slot").innerHTML = '<img src="/red.png?delay=800ms" width="100" height="100">'; "added"`)
		path := filepath.Join(dir, "viewport.png")
		equal(t, "printed path", strings.TrimSpace(ok(t, "screenshot", "--output", path).stdout), path)
		img := readPNG(t, path)
		wantSize(t, "viewport screenshot", img, jsFloat(t, "innerWidth")*dpr, jsFloat(t, "innerHeight")*dpr)
		wantColor(t, "late image", img, 70*dpr, 70*dpr, red)
	})

	t.Run("full page", func(t *testing.T) {
		path := filepath.Join(dir, "full.png")
		ok(t, "screenshot", "--full-page", "--output", path)
		img := readPNG(t, path)
		if h := float64(img.Bounds().Dy()); h < 3000*dpr {
			t.Errorf("full-page screenshot is %.0f high, want the whole %.0f", h, jsFloat(t, "document.documentElement.scrollHeight")*dpr)
		}
		wantColor(t, "box below the fold", img, 150*dpr, 1550*dpr, blue)
	})

	t.Run("selector", func(t *testing.T) {
		path := filepath.Join(dir, "box.png")
		ok(t, "screenshot", "--selector", "#box", "--output", path)
		img := readPNG(t, path)
		wantSize(t, "element screenshot", img, 200*dpr, 100*dpr)
		wantColor(t, "element", img, 100*dpr, 50*dpr, blue)
	})

	contains(t, fails(t, "screenshot", "--full-page", "--selector", "#box"), "mutually exclusive")
}

func TestPDF(t *testing.T) {
	open(t, "/list.html")
	path := filepath.Join(t.TempDir(), "sub", "page.pdf")
	equal(t, "printed path", strings.TrimSpace(ok(t, "pdf", "--format", "letter", "--landscape", "--output", path).stdout), path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Errorf("%s does not start with %%PDF-", path)
	}
	contains(t, fails(t, "pdf", "--format", "b5"), `unknown format "b5"`)
}

func TestDownload(t *testing.T) {
	open(t, "/capture.html")
	path := filepath.Join(t.TempDir(), "sub", "report.txt")
	r := ok(t, "download", "--output", path, "#dl")
	equal(t, "printed path", strings.TrimSpace(r.stdout), path)
	contains(t, r, "downloading: report.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "downloaded file", string(data), "report body")
}

func TestUpload(t *testing.T) {
	open(t, "/capture.html")
	a, _ := filepath.Abs("testdata/upload/a.txt")
	b, _ := filepath.Abs("testdata/upload/b.txt")
	contains(t, ok(t, "upload", "#files", "testdata/upload/a.txt", "testdata/upload/b.txt"), "✓ "+a, "✓ "+b)
	eventually(t, 2*time.Second, "the change event", func() bool { return text(t, "#out") != "" })
	equal(t, "files the page read", text(t, "#out"), "a.txt:first b.txt:second")

	contains(t, fails(t, "upload", "#files", "testdata/upload/missing.txt"), "file not found")
}

func TestDOMStorage(t *testing.T) {
	open(t, "/blank.html")
	js(t, `localStorage.setItem("lk", "lv"); sessionStorage.setItem("sk", "sv"); "set"`)
	origin := "(" + server.URL + ")"

	r := ok(t, "dom-storage")
	contains(t, r, "localStorage  "+origin, "lk = lv", "sessionStorage  "+origin, "sk = sv")

	r = ok(t, "dom-storage", "--local")
	contains(t, r, "lk = lv")
	lacks(t, r, "sessionStorage", "sk = sv")

	r = ok(t, "dom-storage", "--session")
	contains(t, r, "sk = sv")
	lacks(t, r, "localStorage", "lk = lv")

	contains(t, fails(t, "dom-storage", "--local", "--session"), "mutually exclusive")
}

func TestNetwork(t *testing.T) {
	open(t, "/blank.html")
	s := stream(t, "Catching network requests", "network", "--type", "fetch", "--filter", "net", "--show-body")
	equal(t, "status of the watched fetch", js(t, `(await fetch("/list.html?net")).status`), "200")
	equal(t, "status of a filtered fetch", js(t, `(await fetch("/blank.html?other")).status`), "200")
	time.Sleep(2 * time.Second)
	r := s.stop(t)
	contains(t, r, "→ REQ  #1", "GET", "[fetch      ]", "/list.html?net", "← RES  #1", "200 OK", "[BODY  #1   ]", "<title>Products</title>",
		"Stopped. Total requests captured: 1")
	lacks(t, r, "?other", "#2")
}

func TestIntercept(t *testing.T) {
	for _, c := range []struct {
		name        string
		args        []string
		check, want string
		logged      string
	}{
		{
			name:   "block",
			args:   []string{"block", "--url", "*caught*"},
			check:  `await fetch("/list.html?caught").then(() => "loaded", () => "failed")`,
			want:   "failed",
			logged: "✗ BLOCK #1",
		},
		{
			name:   "mock",
			args:   []string{"mock", "--url", "*caught*", "--status", "201", "--body", "hello", "--content-type", "text/plain"},
			check:  `await fetch("/list.html?caught").then(async r => r.status + " " + r.headers.get("content-type") + " " + await r.text())`,
			want:   "201 text/plain hello",
			logged: "★ MOCK  #1",
		},
		{
			name:   "redirect",
			args:   []string{"redirect", "--url", "*caught*", pageURL("/blank.html")},
			check:  `await fetch("/list.html?caught").then(r => r.text()).then(t => t.match(/<title>(.*)<\/title>/)[1])`,
			want:   "Blank",
			logged: "↪ REDIR #1",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			open(t, "/blank.html")
			s := stream(t, "Intercepting requests matching", append([]string{"intercept"}, c.args...)...)
			equal(t, "fetch while intercepting", js(t, c.check), c.want)
			time.Sleep(2 * time.Second)
			r := s.stop(t)
			contains(t, r, "⏸ INT   #1", "/list.html?caught", c.logged, "Stopped. Total requests intercepted: 1")
			equal(t, "title fetched after the stop", js(t, `await fetch("/list.html?caught").then(r => r.text()).then(t => t.match(/<title>(.*)<\/title>/)[1])`), "Products")
		})
	}
}
