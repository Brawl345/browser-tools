package e2e

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const port = "9444"

var (
	binary     string
	profileDir string
	variant    string
	server     *httptest.Server
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	tmp, err := os.MkdirTemp("", "browser-tools-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(tmp)

	binary = filepath.Join(tmp, "browser-tools")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "..")
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "building browser-tools:", err)
		return 1
	}

	server = httptest.NewServer(pageHandler())
	defer server.Close()

	if testing.Short() {
		return m.Run()
	}

	profileDir = filepath.Join(tmp, "profile")
	variant = testBrowser()
	os.Setenv("BROWSER_TOOLS_USER_DATA_DIR", profileDir)
	os.Setenv("BROWSER_TOOLS_NO_UPDATE_CHECK", "1")
	os.Setenv("BROWSER_TOOLS_BROWSER", variant)

	quitBrowser()
	if out, err := exec.Command(binary, "--port", port, "start").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "starting %s on port %s: %v\n%s", variant, port, err, out)
		quitBrowser()
		return 1
	}
	defer quitBrowser()
	return m.Run()
}

func quitBrowser() {
	_ = exec.Command(binary, "--port", port, "stop").Run()
	if profileDir != "" && runtime.GOOS != "windows" {
		_ = exec.Command("pkill", "-f", profileDir).Run()
	}
}

func testBrowser() string {
	if v := os.Getenv("BROWSER_TOOLS_TEST_BROWSER"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".cache", "claude-browser-tools", "chrome-for-testing-binaries", "stable")
	if _, err := os.Stat(filepath.Join(base, ".version")); err == nil {
		return "cft-stable"
	}
	return "chrome-stable"
}

func pageHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("testdata")))
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1500 * time.Millisecond)
		fmt.Fprint(w, "<!doctype html><title>Slow</title><p id=slow>done</p>")
	})
	mux.HandleFunc("/set-cookie", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "server", Value: "1", Path: "/", HttpOnly: true})
		fmt.Fprint(w, "<!doctype html><title>Cookie</title><p>set</p>")
	})
	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="report.txt"`)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "report body")
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/list.html", http.StatusFound)
	})
	return mux
}

func pageURL(name string) string {
	return server.URL + "/" + strings.TrimPrefix(name, "/")
}
