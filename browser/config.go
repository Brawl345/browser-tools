package browser

import (
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	Executable  string
	ProcessName string
}

func GetConfig(variant string) (Config, bool) {
	c, ok := platformConfigs()[variant]
	return c, ok
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
