package main

import (
	"slices"
	"testing"
)

func TestHoistGlobalFlags(t *testing.T) {
	cases := []struct {
		in, want []string
	}{
		{
			[]string{"navigate", "x", "--port", "9333"},
			[]string{"--port", "9333", "navigate", "x"},
		},
		{
			[]string{"wait", "#x", "--timeout=30s"},
			[]string{"--timeout=30s", "wait", "#x"},
		},
		{
			[]string{"element", "li", "--limit", "3"},
			[]string{"element", "li", "--limit", "3"},
		},
		{
			[]string{"--port", "1", "tab"},
			[]string{"--port", "1", "tab"},
		},
		{
			[]string{"mouse", "click", "#a", "--dialog", "dismiss", "--force"},
			[]string{"--dialog", "dismiss", "mouse", "click", "#a", "--force"},
		},
	}
	for _, c := range cases {
		if got := hoistGlobalFlags(c.in); !slices.Equal(got, c.want) {
			t.Errorf("hoistGlobalFlags(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
