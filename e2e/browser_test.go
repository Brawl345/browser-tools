package e2e

import (
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

func startBrowser(t *testing.T, flags ...string) {
	t.Helper()
	ok(t, append([]string{"--timeout", "30s", "start"}, flags...)...)
}

// restart quits the shared browser and starts it with flags; the test's cleanup
// brings it back without them.
func restart(t *testing.T, flags ...string) {
	t.Helper()
	needBrowser(t)
	t.Cleanup(func() {
		ok(t, "stop")
		startBrowser(t)
	})
	ok(t, "stop")
	startBrowser(t, flags...)
}

func portOpen() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func tabStatePath() string {
	base := filepath.Join(os.Getenv("LOCALAPPDATA"), "claude-browser-tools")
	if runtime.GOOS != "windows" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache", "claude-browser-tools")
	}
	return filepath.Join(base, "tabs", port+".json")
}

func TestStartWhileRunning(t *testing.T) {
	reset(t)
	contains(t, ok(t, "start"), "already running on port "+port)
	contains(t, fails(t, "start", "--fake-media"), "already running on port "+port+" and ignores launch flags", "run `browser-tools stop` first")
}

func TestStop(t *testing.T) {
	reset(t)
	if _, err := os.Stat(tabStatePath()); err != nil {
		t.Fatalf("no tab state before stop: %v", err)
	}
	t.Cleanup(func() { startBrowser(t) })

	contains(t, ok(t, "stop"), "browser on port "+port+" quit")
	if portOpen() {
		t.Errorf("port %s is still in use after stop", port)
	}
	if _, err := os.Stat(tabStatePath()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("tab state %s is left after stop: %v", tabStatePath(), err)
	}
	contains(t, ok(t, "stop"), "no browser is running on port "+port)
}

func TestStartFakeMedia(t *testing.T) {
	restart(t, "--fake-media")
	ok(t, "navigate", strings.Replace(pageURL("/blank.html"), "127.0.0.1", "localhost", 1))
	equal(t, "tracks from getUserMedia",
		js(t, `(await navigator.mediaDevices.getUserMedia({video: true, audio: true})).getTracks().map(t => t.kind).sort().join(" ")`),
		"audio video")
}

func needCfT(t *testing.T) {
	t.Helper()
	needBrowser(t)
	if !strings.HasPrefix(variant, "cft-") {
		t.Skipf("extensions need Chrome for Testing, the tests run %s", variant)
	}
}

var extensionIDPattern = regexp.MustCompile(`[a-p]{32}`)

func loadExtension(t *testing.T) string {
	t.Helper()
	r := ok(t, "extension", "load", "testdata/ext")
	contains(t, r, "loaded ")
	id := extensionIDPattern.FindString(r.stdout)
	if id == "" {
		t.Fatal("extension load printed no ID")
	}
	t.Cleanup(func() { run(t, "extension", "uninstall", id) })
	return id
}

func TestExtensionCommands(t *testing.T) {
	needCfT(t)
	reset(t)
	id := loadExtension(t)
	contains(t, ok(t, "extension", "list"), id, "BT Test Extension", "1.0", "enabled")

	ok(t, "navigate", pageURL("/list.html"))
	contains(t, ok(t, "extension", "action", id), "triggered action for "+id)
	eventually(t, 3*time.Second, "the extension's tab", func() bool { return tabCount(t) == 2 })
	contains(t, ok(t, "tab"), "Opened by the extension")
	for range 2 {
		equal(t, "title with the extension's background tab open", js(t, "document.title"), "Products")
	}

	contains(t, ok(t, "extension", "reload", id), "reloaded "+id)
	contains(t, ok(t, "extension", "list"), id)
	contains(t, ok(t, "extension", "uninstall", id), "uninstalled "+id)
	contains(t, ok(t, "extension", "list"), "No unpacked extensions loaded.")
}

// Extensions loaded over CDP are gone after a restart, so the extension comes
// from the command line here.
func TestExtensionCatchesUserScriptRightAfterStart(t *testing.T) {
	needCfT(t)
	dir, _ := filepath.Abs("testdata/ext")
	restart(t, "--arg=--load-extension="+dir)
	ok(t, "navigate", pageURL("/script.user.js"))
	id := extensionIDPattern.FindString(ok(t, "extension", "list").stdout)
	if id == "" {
		t.Fatal("the extension from --load-extension is not listed")
	}
	install := "chrome-extension://" + id + "/install.html"
	eventually(t, 3*time.Second, "the extension's install page", func() bool {
		return strings.HasPrefix(js(t, "location.href"), install)
	})
}
