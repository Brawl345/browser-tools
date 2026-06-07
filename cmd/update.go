package cmd

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	flag "github.com/spf13/pflag"
)

// asset returns the release asset base name and the binary's path inside the
// ZIP for the running OS/arch, matching the names produced by release.yml.
func asset() (name, binInZip string, ok bool) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64":
		return "browser-tools-darwin-amd64", "scripts/browser-tools", true
	case "darwin/arm64":
		return "browser-tools-darwin-arm64", "scripts/browser-tools", true
	case "windows/amd64":
		return "browser-tools-windows-x64", "scripts/browser-tools.exe", true
	case "linux/amd64":
		return "browser-tools-linux-x64", "scripts/browser-tools", true
	case "linux/arm64":
		return "browser-tools-linux-arm64", "scripts/browser-tools", true
	}
	return "", "", false
}

func Update(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	force := fs.Bool("force", false, "update even if already on the latest commit")
	check := fs.Bool("check", false, "only report whether an update is available")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: browser-tools update [--check] [--force]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Updates browser-tools to the latest release, replacing the running")
		fmt.Fprintln(os.Stderr, "binary and the SKILL.md / REFERENCE.md docs in place.")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	assetName, binInZip, ok := asset()
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unsupported platform %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 60 * time.Second}

	remote, err := latestCommit(ctx, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	upToDate := Commit != "dev" && strings.EqualFold(Commit, remote)

	if *check {
		if upToDate {
			fmt.Printf("%s✓%s up to date (%s)\n", ansiGreen, ansiReset, short(remote))
		} else {
			fmt.Printf("%s↑%s update available (%s → %s)\n", ansiYellow, ansiReset, short(Commit), short(remote))
		}
		return
	}

	if upToDate && !*force {
		fmt.Printf("%s✓%s already up to date (%s)\n", ansiGreen, ansiReset, short(remote))
		return
	}

	if Commit == "dev" && !*force {
		fmt.Fprintf(os.Stderr, "%s!%s local build without embedded commit — cannot compare. Use --force to update anyway.\n", ansiYellow, ansiReset)
		return
	}

	url, digest, err := releaseAsset(ctx, client, assetName+".zip")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("downloading %s …\n", assetName)
	data, err := download(ctx, client, url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if digest != "" {
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, digest) {
			fmt.Fprintf(os.Stderr, "error: checksum mismatch (expected %s, got %s)\n", digest, got)
			os.Exit(1)
		}
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid archive: %v\n", err)
		os.Exit(1)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	skillDir := filepath.Dir(filepath.Dir(exe))

	newBin, err := zipEntry(zr, binInZip)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := replaceBinary(exe, newBin); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	for _, doc := range []string{"SKILL.md", "REFERENCE.md"} {
		content, err := zipEntry(zr, doc)
		if err != nil {
			continue // docs are optional
		}
		if err := os.WriteFile(filepath.Join(skillDir, doc), content, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "%s!%s failed to update %s: %v\n", ansiYellow, ansiReset, doc, err)
		}
	}

	fmt.Printf("%s✓%s updated to %s (binary + docs)\n", ansiGreen, ansiReset, short(remote))
}

// latestCommit resolves the commit SHA the lightweight "latest" tag points to.
func latestCommit(ctx context.Context, client *http.Client) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/git/refs/tags/latest", Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github API returned %s", resp.Status)
	}
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ref); err != nil {
		return "", err
	}
	if ref.Object.SHA == "" {
		return "", fmt.Errorf("could not resolve latest tag")
	}
	return ref.Object.SHA, nil
}

// releaseAsset returns the download URL and the bare sha256 digest (without the
// "sha256:" prefix, empty if GitHub has none) for the named asset of the latest
// release.
func releaseAsset(ctx context.Context, client *http.Client, name string) (url, digest string, err error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/latest", Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("github API returned %s", resp.Status)
	}
	var rel struct {
		Assets []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", "", err
	}
	for _, a := range rel.Assets {
		if a.Name == name {
			return a.URL, strings.TrimPrefix(a.Digest, "sha256:"), nil
		}
	}
	return "", "", fmt.Errorf("asset %s not found in latest release", name)
}

func download(ctx context.Context, client *http.Client, url string) ([]byte, error) {
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
	return io.ReadAll(resp.Body)
}

func zipEntry(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("%s not found in archive", name)
}

// replaceBinary swaps the running executable for new contents. On Unix it
// renames a temp file over the target (atomic, old inode stays open). On
// Windows the running exe is moved aside first, since it cannot be overwritten.
func replaceBinary(exe string, content []byte) error {
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".browser-tools-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		return err
	}

	if runtime.GOOS == "windows" {
		old := exe + ".old"
		os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			os.Remove(tmpName)
			return err
		}
		if err := os.Rename(tmpName, exe); err != nil {
			os.Rename(old, exe) // best-effort restore
			return err
		}
		return nil
	}

	if err := os.Rename(tmpName, exe); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
