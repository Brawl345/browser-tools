package browser

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// HolderCommand is the hidden command that keeps a tab's emulation alive.
// Chrome drops every Emulation override once the CDP session that set it
// detaches, so a detached process holds that session open until it is told to
// stop through its PID file, or the tab goes away.
const HolderCommand = "emulation-holder"

// EmulationPIDFile is where the holder for a tab records its PID. Removing the
// file tells the holder to clear the overrides and exit.
func EmulationPIDFile(port int, id TargetID) string {
	return filepath.Join(cacheBase(), "emulation", fmt.Sprintf("%d-%s.pid", port, id))
}

func emulationSpecFile(port int, id TargetID) string {
	return strings.TrimSuffix(EmulationPIDFile(port, id), ".pid") + ".json"
}

// CurrentEmulation returns the spec the running holder of a tab applies, or
// an empty string when the tab is not emulated.
func CurrentEmulation(port int, id TargetID) string {
	pid := ReadPID(EmulationPIDFile(port, id))
	if pid == 0 || !processAlive(pid) {
		return ""
	}
	data, _ := os.ReadFile(emulationSpecFile(port, id))
	return string(data)
}

// ReadPID returns the PID stored in a holder's PID file, or 0.
func ReadPID(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return pid
}

// WritePID atomically records the current process as a tab's holder.
func WritePID(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// StopEmulation ends the holder of a tab and returns once its overrides are
// cleared. It reports whether a holder was running.
func StopEmulation(port int, id TargetID) bool {
	path := EmulationPIDFile(port, id)
	pid := ReadPID(path)
	if pid == 0 {
		return false
	}
	_ = os.Remove(path)
	deadline := time.Now().Add(3 * time.Second)
	for processAlive(pid) {
		if time.Now().After(deadline) {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	return true
}

// StartEmulation launches a holder for a tab and waits until it has applied
// the overrides. spec is handed to the holder verbatim.
func StartEmulation(variant string, port int, id TargetID, spec string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	c := exec.Command(exe, "--browser", variant, "--port", strconv.Itoa(port), HolderCommand, string(id), spec)
	stdout, err := c.StdoutPipe()
	if err != nil {
		return err
	}
	detachProcess(c)
	if err := c.Start(); err != nil {
		return fmt.Errorf("cannot start the emulation holder: %w", err)
	}
	line, _ := bufio.NewReader(stdout).ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "ok" {
		_ = c.Process.Release()
		_ = os.WriteFile(emulationSpecFile(port, id), []byte(spec), 0o644)
		return nil
	}
	_ = c.Process.Kill()
	if line == "" {
		line = "the emulation holder exited without a reply"
	}
	return fmt.Errorf("%s", line)
}
