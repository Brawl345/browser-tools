package e2e

import (
	"testing"
	"time"
)

func TestDialogsOpenedByAClick(t *testing.T) {
	cases := []struct {
		name   string
		flags  []string
		button string
		line   string
		out    string
	}{
		{"alert", nil, "#alert", `dialog: alert "Saved" accepted`, "after alert"},
		{"confirm accepted", nil, "#confirm", `dialog: confirm "Really delete?" accepted`, "confirmed"},
		{"confirm dismissed", []string{"--dialog", "dismiss"}, "#confirm", `dialog: confirm "Really delete?" dismissed`, "cancelled"},
		{"prompt default", nil, "#prompt", `dialog: prompt "Name?" answered "Guest"`, "name=Guest"},
		{"prompt text", []string{"--dialog", "Robin"}, "#prompt", `dialog: prompt "Name?" answered "Robin"`, "name=Robin"},
		{"prompt dismissed", []string{"--dialog", "dismiss"}, "#prompt", `dialog: prompt "Name?" dismissed`, "name=null"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			open(t, "/dialogs.html")
			r := ok(t, append(c.flags, "mouse", "click", c.button)...)
			contains(t, r, c.line)
			equal(t, "#out", text(t, "#out"), c.out)
		})
	}
}

func TestDialogBetweenCommands(t *testing.T) {
	open(t, "/dialogs.html")
	ok(t, "mouse", "click", "#later")
	time.Sleep(time.Second)
	r := ok(t, "element", "#out", "--attr", "text")
	contains(t, r, `dialog: alert "Timer done" was accepted between commands`, "timer answered")
	lacks(t, ok(t, "element", "#out", "--attr", "text"), "between commands")
}
