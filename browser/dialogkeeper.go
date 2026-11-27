package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// KeeperCommand is the hidden command that answers dialogs opening while no
// command runs. Chrome reports a dialog only to sessions that had the Page
// domain enabled when it opened, and `Page.enable` itself blocks until the
// dialog is gone, so a later command could neither see nor close it.
const KeeperCommand = "dialog-keeper"

// keeperGrace leaves a dialog to the command attached to its tab, which answers
// it with that command's --dialog choice.
const keeperGrace = 400 * time.Millisecond

func keeperDir() string { return filepath.Join(cacheBase(), "dialogs") }

func keeperPIDFile(port int) string {
	return filepath.Join(keeperDir(), fmt.Sprintf("%d.pid", port))
}

func keeperLog(port int) string {
	return filepath.Join(keeperDir(), fmt.Sprintf("%d.log", port))
}

type keptDialog struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	URL     string `json:"url"`
}

// EnsureDialogKeeper starts the keeper for a port unless one is running.
func EnsureDialogKeeper(variant string, port int) {
	if pid := ReadPID(keeperPIDFile(port)); pid != 0 && processAlive(pid) {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	c := exec.Command(exe, "--browser", variant, "--port", strconv.Itoa(port), KeeperCommand)
	detachProcess(c)
	if c.Start() == nil {
		_ = c.Process.Release()
	}
}

// ReportKeptDialogs prints and forgets the dialogs the keeper answered since
// the last command.
func ReportKeptDialogs(port int) {
	path := keeperLog(port)
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return
	}
	_ = os.Remove(path)
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var d keptDialog
		if dec.Decode(&d) != nil {
			return
		}
		fmt.Fprintf(os.Stderr, "dialog: %s was accepted between commands (%s)\n", describeDialog(d.Kind, d.Message), d.URL)
	}
}

// RunDialogKeeper attaches to every page of the browser and accepts each
// dialog that no command answers. It exits when the browser goes away.
func RunDialogKeeper(port int) error {
	pidFile := keeperPIDFile(port)
	if pid := ReadPID(pidFile); pid != 0 && pid != os.Getpid() && processAlive(pid) {
		return nil
	}
	if err := WritePID(pidFile); err != nil {
		return err
	}
	defer func() {
		if ReadPID(pidFile) == os.Getpid() {
			_ = os.Remove(pidFile)
		}
	}()

	wsURL, err := browserWSURL(port)
	if err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := chromedp.DialContext(ctx, wsURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	k := &keeper{conn: conn, port: port, pending: map[int64]func(*cdproto.Message){}, attached: map[target.ID]bool{}}
	k.send("", "Target.setDiscoverTargets", map[string]any{"discover": true}, nil)
	for {
		var m cdproto.Message
		if err := conn.Read(ctx, &m); err != nil {
			return nil
		}
		k.handle(&m)
	}
}

type keeper struct {
	conn *chromedp.Conn
	port int
	next atomic.Int64

	mu       sync.Mutex
	pending  map[int64]func(*cdproto.Message)
	attached map[target.ID]bool
}

func (k *keeper) send(session target.SessionID, method string, params any, done func(*cdproto.Message)) {
	raw, _ := json.Marshal(params)
	id := k.next.Add(1)
	k.mu.Lock()
	if done != nil {
		k.pending[id] = done
	}
	_ = k.conn.Write(context.Background(), &cdproto.Message{ID: id, SessionID: session, Method: cdproto.MethodType(method), Params: raw})
	k.mu.Unlock()
}

func (k *keeper) handle(m *cdproto.Message) {
	if m.ID != 0 {
		k.mu.Lock()
		done := k.pending[m.ID]
		delete(k.pending, m.ID)
		k.mu.Unlock()
		if done != nil {
			done(m)
		}
		return
	}
	switch m.Method {
	case "Target.targetCreated", "Target.targetInfoChanged":
		var ev target.EventTargetCreated
		if json.Unmarshal(m.Params, &ev) != nil || ev.TargetInfo == nil {
			return
		}
		k.attach(ev.TargetInfo)
	case "Target.targetDestroyed":
		var ev target.EventTargetDestroyed
		if json.Unmarshal(m.Params, &ev) == nil {
			k.mu.Lock()
			delete(k.attached, ev.TargetID)
			k.mu.Unlock()
		}
	case "Page.javascriptDialogOpening":
		var ev page.EventJavascriptDialogOpening
		if json.Unmarshal(m.Params, &ev) != nil {
			return
		}
		session := m.SessionID
		time.AfterFunc(keeperGrace, func() { k.answer(session, &ev) })
	}
}

func (k *keeper) attach(t *target.Info) {
	if t.Type != "page" {
		return
	}
	k.mu.Lock()
	if k.attached[t.TargetID] {
		k.mu.Unlock()
		return
	}
	k.attached[t.TargetID] = true
	k.mu.Unlock()
	k.send("", "Target.attachToTarget", map[string]any{"targetId": t.TargetID, "flatten": true}, func(m *cdproto.Message) {
		var res target.AttachToTargetReturns
		if m.Error != nil || json.Unmarshal(m.Result, &res) != nil {
			return
		}
		k.send(res.SessionID, "Page.enable", nil, nil)
	})
}

// answer accepts a dialog that is still open after the grace period. A dialog
// a command already answered makes Chrome reply with an error, which is fine.
func (k *keeper) answer(session target.SessionID, ev *page.EventJavascriptDialogOpening) {
	params := map[string]any{"accept": true}
	if ev.Type == page.DialogTypePrompt {
		params["promptText"] = ev.DefaultPrompt
	}
	k.send(session, "Page.handleJavaScriptDialog", params, func(m *cdproto.Message) {
		if m.Error != nil {
			return
		}
		raw, _ := json.Marshal(keptDialog{Kind: string(ev.Type), Message: ev.Message, URL: ev.URL})
		f, err := os.OpenFile(keeperLog(k.port), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		_, _ = f.Write(append(raw, '\n'))
		_ = f.Close()
	})
}

func browserWSURL(port int) (string, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	return v.WebSocketDebuggerURL, nil
}
