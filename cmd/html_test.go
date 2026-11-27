package cmd

import "testing"

func TestFilterWithContext(t *testing.T) {
	text := "a\nb\nmatch 1\nc\nd\ne\nf\nmatch 2\ng"
	cases := []struct {
		pattern string
		context int
		want    string
	}{
		{"match", 0, "match 1\n--\nmatch 2\n"},
		{"match", 1, "b\nmatch 1\nc\n--\nf\nmatch 2\ng\n"},
		{"match", 2, "a\nb\nmatch 1\nc\nd\ne\nf\nmatch 2\ng\n"},
		{"nothing", 3, ""},
	}
	for _, c := range cases {
		got, err := filterWithContext(text, c.pattern, c.context)
		if err != nil || got != c.want {
			t.Errorf("filterWithContext(%q, %d) = %q, %v, want %q", c.pattern, c.context, got, err, c.want)
		}
	}
	if _, err := filterWithContext(text, "(", 0); err == nil {
		t.Error("an invalid regex was accepted")
	}
}
