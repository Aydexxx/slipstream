package privatemode

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

// amneziawg-go (like wireguard-go) exposes a UAPI over a per-interface named
// pipe, secured to Administrators. Since the app runs elevated we can open it
// and read live tunnel state — most importantly the last-handshake time, which
// is the ground truth for "is the tunnel actually up".
const uapiPipePrefix = `\\.\pipe\ProtectedPrefix\Administrators\AmneziaWG\`

// handshakeInfo is the subset of the UAPI dump we care about.
type handshakeInfo struct {
	HasPeer       bool
	LastHandshake time.Time // zero if never handshaked
	RxBytes       int64
	TxBytes       int64
	Endpoint      string
}

// Age returns how long ago the last handshake happened, or -1 if there has
// never been one.
func (h handshakeInfo) Age(now time.Time) time.Duration {
	if h.LastHandshake.IsZero() {
		return -1
	}
	return now.Sub(h.LastHandshake)
}

// queryHandshake opens the tunnel's UAPI pipe, requests a config dump, and
// parses the handshake state. A missing pipe (service still starting, or down)
// returns an error the caller treats as "not up yet", not a hard failure.
func queryHandshake(ifaceName string) (handshakeInfo, error) {
	path := uapiPipePrefix + ifaceName
	p16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return handshakeInfo{}, err
	}
	h, err := windows.CreateFile(
		p16,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil, windows.OPEN_EXISTING, 0, 0,
	)
	if err != nil {
		return handshakeInfo{}, fmt.Errorf("open UAPI pipe: %w", err)
	}
	f := os.NewFile(uintptr(h), path)
	defer f.Close()

	if _, err := f.Write([]byte("get=1\n\n")); err != nil {
		return handshakeInfo{}, fmt.Errorf("UAPI request: %w", err)
	}
	return parseUAPI(f)
}

// parseUAPI reads a wireguard/amneziawg UAPI "get" response and extracts
// handshake state. The response is newline-separated "key=value" pairs,
// terminated by a blank line; we stop there so we don't block waiting for the
// server's next command.
func parseUAPI(r io.Reader) (handshakeInfo, error) {
	var (
		info      handshakeInfo
		sec, nsec int64
		errno     = "0"
	)
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			// Blank line terminates the response (or EOF).
			if err != nil && err != io.EOF && len(line) == 0 {
				break
			}
			break
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			if err == io.EOF {
				break
			}
			continue
		}
		switch key {
		case "public_key":
			info.HasPeer = true
		case "endpoint":
			info.Endpoint = value
		case "last_handshake_time_sec":
			sec, _ = strconv.ParseInt(value, 10, 64)
		case "last_handshake_time_nsec":
			nsec, _ = strconv.ParseInt(value, 10, 64)
		case "rx_bytes":
			info.RxBytes, _ = strconv.ParseInt(value, 10, 64)
		case "tx_bytes":
			info.TxBytes, _ = strconv.ParseInt(value, 10, 64)
		case "errno":
			errno = value
		}

		if err == io.EOF {
			break
		}
	}

	if errno != "0" {
		return info, fmt.Errorf("UAPI reported errno=%s", errno)
	}
	if sec > 0 {
		info.LastHandshake = time.Unix(sec, nsec)
	}
	return info, nil
}
