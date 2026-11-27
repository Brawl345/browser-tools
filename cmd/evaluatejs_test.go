package cmd

import (
	"testing"

	"github.com/chromedp/cdproto/runtime"
)

func TestFormatResult(t *testing.T) {
	cases := []struct {
		name string
		obj  *runtime.RemoteObject
		want string
	}{
		{"nil", nil, ""},
		{"undefined", &runtime.RemoteObject{Type: runtime.TypeUndefined}, ""},
		{"string unquoted", &runtime.RemoteObject{Type: runtime.TypeString, Value: []byte(`"a \"b\""`)}, `a "b"`},
		{"number", &runtime.RemoteObject{Type: runtime.TypeNumber, Value: []byte(`2`)}, "2"},
		{"object as JSON", &runtime.RemoteObject{Type: runtime.TypeObject, Value: []byte(`{"a":[1,2]}`)}, `{"a":[1,2]}`},
		{"null", &runtime.RemoteObject{Type: runtime.TypeObject, Subtype: runtime.SubtypeNull, Value: []byte(`null`)}, "null"},
		{"unserializable", &runtime.RemoteObject{Type: runtime.TypeNumber, UnserializableValue: "Infinity"}, "Infinity"},
		{"node description", &runtime.RemoteObject{Type: runtime.TypeObject, Subtype: runtime.SubtypeNode, Description: "h1.title"}, "h1.title"},
	}
	for _, c := range cases {
		if got := formatResult(c.obj); got != c.want {
			t.Errorf("%s: formatResult = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestIsIllegalReturn(t *testing.T) {
	exc := &runtime.ExceptionDetails{
		Text:      "Uncaught",
		Exception: &runtime.RemoteObject{Description: "SyntaxError: Illegal return statement"},
	}
	if !isIllegalReturn(exc) {
		t.Error("an illegal return was not recognised")
	}
	other := &runtime.ExceptionDetails{Exception: &runtime.RemoteObject{Description: "Error: boom"}}
	if isIllegalReturn(other) {
		t.Error("a plain error counted as an illegal return")
	}
}
