package browser

import (
	"context"
	"fmt"
	"os"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// DialogAnswer is how alert, confirm, prompt and beforeunload dialogs are
// answered: "accept", "dismiss", or a text that accepts a prompt with it.
// Empty leaves dialogs alone.
var DialogAnswer string

// answerDialogs closes every dialog the tab opens while this process is
// attached, so the page and the command waiting on it carry on.
func answerDialogs(tabCtx context.Context) {
	chromedp.ListenTarget(tabCtx, func(ev any) {
		if e, ok := ev.(*page.EventJavascriptDialogOpening); ok {
			go answerDialog(tabCtx, e.Type, e.Message, e.DefaultPrompt)
		}
	})
}

func answerDialog(tabCtx context.Context, kind page.DialogType, message, defaultPrompt string) {
	accept := DialogAnswer != "dismiss"
	p := page.HandleJavaScriptDialog(accept)
	verdict := "dismissed"
	if accept {
		verdict = "accepted"
		if kind == page.DialogTypePrompt {
			text := defaultPrompt
			if DialogAnswer != "accept" {
				text = DialogAnswer
			}
			p = p.WithPromptText(text)
			verdict = fmt.Sprintf("answered %q", text)
		}
	}
	if err := chromedp.Run(tabCtx, p); err != nil {
		fmt.Fprintf(os.Stderr, "dialog: %s could not be closed: %v\n", describeDialog(string(kind), message), err)
		return
	}
	fmt.Fprintf(os.Stderr, "dialog: %s %s\n", describeDialog(string(kind), message), verdict)
}

func describeDialog(kind, message string) string {
	if message == "" {
		return kind
	}
	return fmt.Sprintf("%s %q", kind, message)
}
