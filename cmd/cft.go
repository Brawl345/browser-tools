package cmd

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"browser-tools/browser"
	flag "github.com/spf13/pflag"
)

const cftVersionsURL = "https://googlechromelabs.github.io/chrome-for-testing/last-known-good-versions-with-downloads.json"

var cftValidChannels = map[string]bool{"stable": true, "beta": true, "dev": true, "canary": true}

func CfT(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("update-cft", flag.ExitOnError)
	channel := fs.String("channel", "stable", "channel to install: stable, beta, dev, canary")
	check := fs.Bool("check", false, "only report whether an update is available")
	force := fs.Bool("force", false, "reinstall even if already up to date")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools update-cft [--channel <c>] [--check] [--force]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Downloads or updates a Chrome for Testing build into the local cache.")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	ch := strings.ToLower(*channel)
	if !cftValidChannels[ch] {
		fmt.Fprintf(os.Stderr, "error: unknown channel %q (stable, beta, dev, canary)\n", *channel)
		os.Exit(1)
	}

	plat, ok := browser.CfTPlatform()
	if !ok {
		fmt.Fprintln(os.Stderr, "error: Chrome for Testing is not available for this platform")
		os.Exit(1)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	version, url, err := cftLatest(ctx, client, ch, plat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	dir := browser.ChromeForTestingDir(ch)
	installed := cftInstalledVersion(dir)
	upToDate := installed == version

	if *check {
		if upToDate {
			fmt.Printf("%s✓%s %s up to date (%s)\n", ansiGreen, ansiReset, ch, version)
		} else if installed == "" {
			fmt.Printf("%s↑%s %s not installed (latest %s)\n", ansiYellow, ansiReset, ch, version)
		} else {
			fmt.Printf("%s↑%s %s update available (%s → %s)\n", ansiYellow, ansiReset, ch, installed, version)
		}
		return
	}

	if upToDate && !*force {
		fmt.Printf("%s✓%s %s already up to date (%s)\n", ansiGreen, ansiReset, ch, version)
		return
	}

	fmt.Printf("downloading Chrome for Testing %s %s (%s) …\n", ch, version, plat)
	data, err := downloadWithProgress(ctx, client, url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := extractZip(data, dir); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(dir, ".version"), []byte(version), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "%s!%s could not record version: %v\n", ansiYellow, ansiReset, err)
	}

	fmt.Printf("%s✓%s installed %s %s → use --browser cft-%s\n", ansiGreen, ansiReset, ch, version, ch)
}

func cftInstalledVersion(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, ".version"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// cftLatest resolves the latest version and chrome download URL for a channel.
func cftLatest(ctx context.Context, client *http.Client, channel, platform string) (version, url string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cftVersionsURL, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("version feed returned %s", resp.Status)
	}

	var feed struct {
		Channels map[string]struct {
			Version   string `json:"version"`
			Downloads struct {
				Chrome []struct {
					Platform string `json:"platform"`
					URL      string `json:"url"`
				} `json:"chrome"`
			} `json:"downloads"`
		} `json:"channels"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return "", "", err
	}

	key := strings.ToUpper(channel[:1]) + channel[1:]
	c, ok := feed.Channels[key]
	if !ok {
		return "", "", fmt.Errorf("channel %q not found in feed", channel)
	}
	for _, d := range c.Downloads.Chrome {
		if d.Platform == platform {
			return c.Version, d.URL, nil
		}
	}
	return "", "", fmt.Errorf("no %s build for platform %s", channel, platform)
}

// extractZip writes every entry of a ZIP archive under dest, preserving file
// modes, directories and symlinks (the latter are required by .app bundles).
func extractZip(data []byte, dest string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid archive: %w", err)
	}
	total := len(zr.File)
	tty := isTTY()
	var last time.Time
	for i, f := range zr.File {
		if tty && (time.Since(last) > 80*time.Millisecond || i == total-1) {
			renderBar("extract", float64(i+1)/float64(total), fmt.Sprintf("%d/%d files", i+1, total))
			last = time.Now()
		}
		target := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("archive entry escapes destination: %s", f.Name)
		}
		info := f.FileInfo()
		switch {
		case info.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case info.Mode()&os.ModeSymlink != 0:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			link, err := readZipEntry(f)
			if err != nil {
				return err
			}
			os.Remove(target)
			if err := os.Symlink(string(link), target); err != nil {
				return err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			content, err := readZipEntry(f)
			if err != nil {
				return err
			}
			if err := os.WriteFile(target, content, info.Mode().Perm()); err != nil {
				return err
			}
		}
	}
	if tty {
		fmt.Fprintln(os.Stderr)
	}
	return nil
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// downloadWithProgress fetches url, rendering a progress bar when attached to a
// terminal and the server reports a content length.
func downloadWithProgress(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: %s", resp.Status)
	}

	if !isTTY() || resp.ContentLength <= 0 {
		return io.ReadAll(resp.Body)
	}

	pr := &progressReader{r: resp.Body, total: resp.ContentLength}
	data, err := io.ReadAll(pr)
	pr.finish()
	return data, err
}

type progressReader struct {
	r     io.Reader
	total int64
	read  int64
	last  time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if time.Since(p.last) > 80*time.Millisecond {
		renderBar("download", float64(p.read)/float64(p.total), fmt.Sprintf("%.1f/%.1f MB", mib(p.read), mib(p.total)))
		p.last = time.Now()
	}
	return n, err
}

func (p *progressReader) finish() {
	renderBar("download", 1, fmt.Sprintf("%.1f/%.1f MB", mib(p.total), mib(p.total)))
	fmt.Fprintln(os.Stderr)
}

func renderBar(label string, frac float64, suffix string) {
	const width = 24
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * width)
	bar := strings.Repeat("=", filled) + strings.Repeat(" ", width-filled)
	fmt.Fprintf(os.Stderr, "\r  %-8s [%s] %3.0f%%  %s", label, bar, frac*100, suffix)
}

func mib(n int64) float64 { return float64(n) / (1024 * 1024) }

func isTTY() bool {
	fi, err := os.Stderr.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
