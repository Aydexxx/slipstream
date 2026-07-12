package privatemode

import (
	"strings"
	"testing"
	"time"
)

func TestParseUAPIConnected(t *testing.T) {
	dump := strings.Join([]string{
		"private_key=0000000000000000000000000000000000000000000000000000000000000000",
		"listen_port=51820",
		"public_key=1111111111111111111111111111111111111111111111111111111111111111",
		"endpoint=203.0.113.9:51820",
		"last_handshake_time_sec=1700000000",
		"last_handshake_time_nsec=500",
		"tx_bytes=4096",
		"rx_bytes=8192",
		"errno=0",
		"",
		"",
	}, "\n")

	info, err := parseUAPI(strings.NewReader(dump))
	if err != nil {
		t.Fatalf("parseUAPI: %v", err)
	}
	if !info.HasPeer {
		t.Error("expected HasPeer")
	}
	if info.LastHandshake.Unix() != 1700000000 {
		t.Errorf("last handshake = %v", info.LastHandshake)
	}
	if info.RxBytes != 8192 || info.TxBytes != 4096 {
		t.Errorf("rx/tx = %d/%d", info.RxBytes, info.TxBytes)
	}
	if info.Endpoint != "203.0.113.9:51820" {
		t.Errorf("endpoint = %q", info.Endpoint)
	}
	if age := info.Age(time.Unix(1700000030, 0)); age.Round(time.Second) != 30*time.Second {
		t.Errorf("age = %v, want ~30s", age)
	}
}

func TestParseUAPINoHandshake(t *testing.T) {
	dump := "public_key=1111\nlast_handshake_time_sec=0\nlast_handshake_time_nsec=0\nerrno=0\n\n"
	info, err := parseUAPI(strings.NewReader(dump))
	if err != nil {
		t.Fatalf("parseUAPI: %v", err)
	}
	if !info.HasPeer {
		t.Error("expected HasPeer even with no handshake")
	}
	if !info.LastHandshake.IsZero() {
		t.Error("expected zero LastHandshake")
	}
	if info.Age(time.Now()) != -1 {
		t.Error("Age with no handshake should be -1")
	}
}

func TestParseUAPIErrno(t *testing.T) {
	if _, err := parseUAPI(strings.NewReader("errno=1\n\n")); err == nil {
		t.Fatal("expected error for errno != 0")
	}
}
