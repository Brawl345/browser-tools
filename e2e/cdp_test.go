package e2e

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	be "encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// Stands in for an extension, which opens tabs no browser-tools command opens.
func cdpBrowser(t *testing.T, method string, params any) json.RawMessage {
	t.Helper()
	res, err := cdpCall(method, params)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return res
}

func cdpCall(method string, params any) (json.RawMessage, error) {
	resp, err := http.Get("http://127.0.0.1:" + port + "/json/version")
	if err != nil {
		return nil, err
	}
	var version struct {
		URL string `json:"webSocketDebuggerUrl"`
	}
	err = json.NewDecoder(resp.Body).Decode(&version)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(version.URL)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTimeout("tcp", u.Host, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	key := make([]byte, 16)
	_, _ = rand.Read(key)
	fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		u.RequestURI(), u.Host, base64.StdEncoding.EncodeToString(key))
	br := bufio.NewReader(conn)
	hs, err := http.ReadResponse(br, nil)
	if err != nil {
		return nil, err
	}
	if hs.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("websocket handshake: %s", hs.Status)
	}

	msg, _ := json.Marshal(map[string]any{"id": 1, "method": method, "params": params})
	if err := writeFrame(conn, msg); err != nil {
		return nil, err
	}
	for {
		data, err := readMessage(br)
		if err != nil {
			return nil, err
		}
		var reply struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &reply) != nil || reply.ID != 1 {
			continue
		}
		if reply.Error != nil {
			return nil, fmt.Errorf("%s", reply.Error.Message)
		}
		return reply.Result, nil
	}
}

func writeFrame(w io.Writer, payload []byte) error {
	header := []byte{0x81}
	switch n := len(payload); {
	case n < 126:
		header = append(header, 0x80|byte(n))
	case n <= 0xffff:
		header = append(header, 0x80|126)
		header = be.BigEndian.AppendUint16(header, uint16(n))
	default:
		header = append(header, 0x80|127)
		header = be.BigEndian.AppendUint64(header, uint64(n))
	}
	mask := make([]byte, 4)
	_, _ = rand.Read(mask)
	header = append(header, mask...)
	masked := make([]byte, len(payload))
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	_, err := w.Write(append(header, masked...))
	return err
}

func readMessage(r *bufio.Reader) ([]byte, error) {
	var msg []byte
	for {
		var head [2]byte
		if _, err := io.ReadFull(r, head[:]); err != nil {
			return nil, err
		}
		n := uint64(head[1] & 0x7f)
		switch n {
		case 126:
			var ext [2]byte
			if _, err := io.ReadFull(r, ext[:]); err != nil {
				return nil, err
			}
			n = uint64(be.BigEndian.Uint16(ext[:]))
		case 127:
			var ext [8]byte
			if _, err := io.ReadFull(r, ext[:]); err != nil {
				return nil, err
			}
			n = be.BigEndian.Uint64(ext[:])
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
		msg = append(msg, payload...)
		if head[0]&0x80 != 0 {
			return msg, nil
		}
	}
}
