package cmd

import (
	"testing"
	"time"
)

func TestHostOf(t *testing.T) {
	cases := map[string]string{
		"https://example.com/path":      "example.com",
		"http://localhost:8080/":        "localhost",
		"https://a.b.example.org?x=1":   "a.b.example.org",
		"https://example.com":           "example.com",
		"http://127.0.0.1:9000/a/b?c#d": "127.0.0.1",
	}
	for u, want := range cases {
		if got := hostOf(u); got != want {
			t.Errorf("hostOf(%q) = %q, want %q", u, got, want)
		}
	}
}

func TestParseExpires(t *testing.T) {
	now := time.Now()
	near := func(got time.Time, want time.Time) bool {
		d := got.Sub(want)
		return d > -time.Minute && d < time.Minute
	}

	cases := []struct {
		in   string
		want time.Time
	}{
		{"7d", now.Add(7 * 24 * time.Hour)},
		{"1.5d", now.Add(36 * time.Hour)},
		{"12h", now.Add(12 * time.Hour)},
		{"30m", now.Add(30 * time.Minute)},
		{"1900000000", time.Unix(1900000000, 0)},
		{"2027-01-31T10:00:00Z", time.Date(2027, 1, 31, 10, 0, 0, 0, time.UTC)},
		{"2027-01-31", time.Date(2027, 1, 31, 0, 0, 0, 0, time.Local)},
	}
	for _, c := range cases {
		got, err := parseExpires(c.in)
		if err != nil {
			t.Errorf("parseExpires(%q): %v", c.in, err)
			continue
		}
		if got == nil || !near(time.Time(*got), c.want) {
			t.Errorf("parseExpires(%q) = %v, want about %v", c.in, got, c.want)
		}
	}

	if got, err := parseExpires("Session"); err != nil || got != nil {
		t.Errorf("parseExpires(Session) = %v, %v, want nil, nil", got, err)
	}
	for _, bad := range []string{"soon", "xd", ""} {
		if _, err := parseExpires(bad); err == nil {
			t.Errorf("parseExpires(%q) accepted", bad)
		}
	}
}
