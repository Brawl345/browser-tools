package browser

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	Executable  string
	ProcessName string
}

// cftChannels maps a Chrome for Testing variant to its channel name.
var cftChannels = map[string]string{
	"cft-stable": "stable",
	"cft-beta":   "beta",
	"cft-dev":    "dev",
	"cft-canary": "canary",
}

// IsChromeForTesting reports whether the variant is a Chrome for Testing build.
func IsChromeForTesting(variant string) bool {
	_, ok := cftChannels[variant]
	return ok
}

// CfTChannel returns the channel ("stable", "beta", …) for a CfT variant.
func CfTChannel(variant string) (string, bool) {
	c, ok := cftChannels[variant]
	return c, ok
}

func GetConfig(variant string) (Config, bool) {
	if channel, ok := cftChannels[variant]; ok {
		exe := CfTExecutable(channel)
		if exe == "" {
			return Config{}, false
		}
		return Config{Executable: exe, ProcessName: "Google Chrome for Testing"}, true
	}
	c, ok := platformConfigs()[variant]
	return c, ok
}

// cacheBase is the root cache directory shared by user data dirs and binaries.
func cacheBase() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "claude-browser-tools")
	default:
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".cache", "claude-browser-tools")
	}
}

// CfTPlatform returns the Chrome for Testing platform identifier, which is also
// the name of the top-level folder inside the downloaded archive.
func CfTPlatform() (string, bool) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		return "mac-arm64", true
	case "darwin/amd64":
		return "mac-x64", true
	case "linux/amd64":
		return "linux64", true
	case "windows/amd64":
		return "win64", true
	case "windows/386":
		return "win32", true
	}
	return "", false
}

// ChromeForTestingDir is the install directory for a Chrome for Testing channel.
func ChromeForTestingDir(channel string) string {
	return filepath.Join(cacheBase(), "chrome-for-testing-binaries", strings.ToLower(channel))
}

// CfTExecutable returns the path to the Chrome for Testing binary for a channel.
func CfTExecutable(channel string) string {
	plat, ok := CfTPlatform()
	if !ok {
		return ""
	}
	base := filepath.Join(ChromeForTestingDir(channel), "chrome-"+plat)
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(base, "Google Chrome for Testing.app", "Contents", "MacOS", "Google Chrome for Testing")
	case "windows":
		return filepath.Join(base, "chrome.exe")
	default:
		return filepath.Join(base, "chrome")
	}
}

func platformConfigs() map[string]Config {
	switch runtime.GOOS {
	case "darwin":
		return map[string]Config{
			"chrome-stable": {
				Executable:  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
				ProcessName: "Google Chrome",
			},
			"chrome-beta": {
				Executable:  "/Applications/Google Chrome Beta.app/Contents/MacOS/Google Chrome Beta",
				ProcessName: "Google Chrome Beta",
			},
			"chrome-dev": {
				Executable:  "/Applications/Google Chrome Dev.app/Contents/MacOS/Google Chrome Dev",
				ProcessName: "Google Chrome Dev",
			},
			"chrome-canary": {
				Executable:  "/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
				ProcessName: "Google Chrome Canary",
			},
		}
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		programFiles := os.Getenv("PROGRAMFILES")
		if programFiles == "" {
			programFiles = `C:\Program Files`
		}
		return map[string]Config{
			"chrome-stable": {
				Executable:  filepath.Join(programFiles, `Google\Chrome\Application\chrome.exe`),
				ProcessName: "chrome.exe",
			},
			"chrome-beta": {
				Executable:  filepath.Join(programFiles, `Google\Chrome Beta\Application\chrome.exe`),
				ProcessName: "chrome.exe",
			},
			"chrome-dev": {
				Executable:  filepath.Join(localAppData, `Google\Chrome Dev\Application\chrome.exe`),
				ProcessName: "chrome.exe",
			},
			"chrome-canary": {
				Executable:  filepath.Join(localAppData, `Google\Chrome SxS\Application\chrome.exe`),
				ProcessName: "chrome.exe",
			},
		}
	default:
		return map[string]Config{
			"chrome-stable": {
				Executable:  "/usr/bin/google-chrome",
				ProcessName: "chrome",
			},
			"chrome-beta": {
				Executable:  "/usr/bin/google-chrome-beta",
				ProcessName: "chrome",
			},
			"chrome-dev": {
				Executable:  "/usr/bin/google-chrome-unstable",
				ProcessName: "chrome",
			},
			"chrome-canary": {
				Executable:  "/usr/bin/google-chrome-canary",
				ProcessName: "chrome",
			},
		}
	}
}

func UserDataDir(variant string) string {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = filepath.Join(os.Getenv("LOCALAPPDATA"), "claude-browser-tools")
	default:
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache", "claude-browser-tools")
	}
	return filepath.Join(base, variant)
}
