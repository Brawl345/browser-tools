package browser

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const frontPIDScript = `ObjC.import("AppKit"); $.NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier`

// restoreFrontScript re-activates the app with the given PID until it has
// stayed frontmost for a moment, since Chrome raises its window asynchronously
// after answering the CDP call. It talks to AppKit directly, so no Apple Events
// permission is involved.
const restoreFrontScript = `ObjC.import("AppKit");
function run(argv) {
	const pid = parseInt(argv[0], 10);
	const app = $.NSRunningApplication.runningApplicationWithProcessIdentifier(pid);
	if (!app || app.isNil()) return;
	const ws = $.NSWorkspace.sharedWorkspace;
	const deadline = Date.now() + parseInt(argv[1], 10);
	let stable = 0;
	while (Date.now() < deadline) {
		$.NSRunLoop.currentRunLoop.runUntilDate($.NSDate.dateWithTimeIntervalSinceNow(0.03));
		if (ws.frontmostApplication.processIdentifier === pid) {
			if (++stable >= 5) return;
		} else {
			stable = 0;
			app.activateWithOptions($.NSApplicationActivateIgnoringOtherApps);
		}
	}
}`

func frontPID() int {
	out, err := exec.Command("osascript", "-l", "JavaScript", "-e", frontPIDScript).Output()
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return pid
}

// KeepFront runs fn, which may bring the browser to the front, and hands the
// focus back to the app that was frontmost before. watch bounds how long the
// browser gets to raise itself after fn returns.
func KeepFront(watch time.Duration, fn func() error) error {
	pid := frontPID()
	err := fn()
	if pid > 0 {
		_ = exec.Command("osascript", "-l", "JavaScript", "-e", restoreFrontScript,
			strconv.Itoa(pid), strconv.FormatInt(watch.Milliseconds(), 10)).Run()
	}
	return err
}
