package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// CfTCheckInterval is how often a browser launch looks for a newer Chrome for
// Testing build.
const CfTCheckInterval = 24 * time.Hour

// NoUpdateCheckEnv turns the background update check off when set to anything but 0.
const NoUpdateCheckEnv = "BROWSER_TOOLS_NO_UPDATE_CHECK"

// cftRetryAfter is how soon a failed background update is tried again.
const cftRetryAfter = time.Hour

// CfTStagingDir holds a downloaded build until the browser is not running, so
// a running browser never loses its files.
func CfTStagingDir(channel string) string {
	return ChromeForTestingDir(channel) + ".new"
}

func cftStampFile(channel string) string {
	return ChromeForTestingDir(channel) + ".last-check"
}

// CfTVersion returns the version recorded in a build directory.
func CfTVersion(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, ".version"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// CfTInUse reports whether a browser runs from the channel's build.
func CfTInUse(channel string) bool {
	if runtime.GOOS == "windows" {
		return false
	}
	out, err := exec.Command("ps", "-Ao", "args").Output()
	if err != nil {
		return true
	}
	return strings.Contains(string(out), ChromeForTestingDir(channel)+string(os.PathSeparator))
}

// ApplyStagedCfT moves a staged build into place unless a browser still runs
// from the current one. It returns the version that was put in place.
func ApplyStagedCfT(channel string) (string, error) {
	staged := CfTStagingDir(channel)
	version := CfTVersion(staged)
	if version == "" || CfTInUse(channel) {
		return "", nil
	}
	dir := ChromeForTestingDir(channel)
	old := dir + ".old"
	_ = os.RemoveAll(old)
	if _, err := os.Stat(dir); err == nil {
		if err := os.Rename(dir, old); err != nil {
			// Windows refuses to move a directory whose binary is running.
			return "", nil
		}
	}
	if err := os.Rename(staged, dir); err != nil {
		_ = os.Rename(old, dir)
		return "", err
	}
	_ = os.RemoveAll(old)
	return version, nil
}

// cftNoticeInterval is how often a command reminds that a downloaded update
// waits for a browser restart.
const cftNoticeInterval = time.Hour

// CfTUpkeep runs on every command: it starts the daily background update check
// and points out a downloaded update that waits for a restart.
func CfTUpkeep(channel string) {
	scheduleCfTUpdate(channel)
	staged := CfTVersion(CfTStagingDir(channel))
	if staged == "" {
		return
	}
	notice := ChromeForTestingDir(channel) + ".notified"
	if info, err := os.Stat(notice); err == nil && time.Since(info.ModTime()) < cftNoticeInterval {
		return
	}
	_ = os.WriteFile(notice, nil, 0o644)
	fmt.Fprintf(os.Stderr, "note: Chrome for Testing %s is downloaded; `browser-tools stop` restarts the browser with it\n", staged)
}

// scheduleCfTUpdate starts a background update check at most once per
// CfTCheckInterval. The download lands in the staging directory.
func scheduleCfTUpdate(channel string) {
	if v := os.Getenv(NoUpdateCheckEnv); v != "" && v != "0" {
		return
	}
	stamp := cftStampFile(channel)
	if info, err := os.Stat(stamp); err == nil && time.Since(info.ModTime()) < CfTCheckInterval {
		return
	}
	if err := os.WriteFile(stamp, nil, 0o644); err != nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	c := exec.Command(exe, "update-cft", "--channel", channel, "--background")
	detachProcess(c)
	_ = c.Start()
}

// RetryCfTUpdateSoon makes the next launch after cftRetryAfter check again.
func RetryCfTUpdateSoon(channel string) {
	t := time.Now().Add(cftRetryAfter - CfTCheckInterval)
	_ = os.Chtimes(cftStampFile(channel), t, t)
}
