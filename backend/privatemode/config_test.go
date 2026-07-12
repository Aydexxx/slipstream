package privatemode

import (
	"reflect"
	"strings"
	"testing"
)

const goodConfig = `[Interface]
PrivateKey = qL9c0m9k3n4Xr2+abcdefghijklmnopqrstuvwx==
Address = 10.66.66.2/32, fd42:42:42::2/128
DNS = 1.1.1.1
MTU = 1280
Jc = 4
Jmin = 40
Jmax = 70
S1 = 86
S2 = 574
H1 = 1148643509
H2 = 1637895918
H3 = 2059136377
H4 = 1051215432
I1 = <b 0xc300000001><r 8>

[Peer]
PublicKey = Srv+PublicKeyBase64000000000000000000000000=
PresharedKey = Psk+Base64000000000000000000000000000000000=
Endpoint = 203.0.113.9:51820
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
`

func TestParseConfigGood(t *testing.T) {
	c, err := ParseConfig(goodConfig)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if c.Endpoint != "203.0.113.9:51820" || c.EndpointHost != "203.0.113.9" {
		t.Errorf("endpoint = %q / host %q", c.Endpoint, c.EndpointHost)
	}
	if !c.FullTunnel {
		t.Error("expected FullTunnel (AllowedIPs has 0.0.0.0/0)")
	}
	if !c.Obfuscated {
		t.Error("expected Obfuscated (Jc.. present)")
	}
	if !c.HasDNS || !reflect.DeepEqual(c.DNS, []string{"1.1.1.1"}) {
		t.Errorf("dns = %v", c.DNS)
	}
	if !reflect.DeepEqual(c.InterfaceAddresses, []string{"10.66.66.2/32", "fd42:42:42::2/128"}) {
		t.Errorf("addresses = %v", c.InterfaceAddresses)
	}
	if c.PeerPublicKey == "" {
		t.Error("missing peer public key")
	}
}

func TestParseConfigMissingFields(t *testing.T) {
	cases := map[string]string{
		"no interface": "[Peer]\nPublicKey = x\nEndpoint = 1.2.3.4:5\nAllowedIPs = 0.0.0.0/0\n",
		"no endpoint":  "[Interface]\nPrivateKey = x\nAddress = 10.0.0.2/32\n[Peer]\nPublicKey = y\nAllowedIPs = 0.0.0.0/0\n",
		"no privkey":   "[Interface]\nAddress = 10.0.0.2/32\n[Peer]\nPublicKey = y\nEndpoint = 1.2.3.4:5\nAllowedIPs = 0.0.0.0/0\n",
	}
	for name, cfg := range cases {
		if _, err := ParseConfig(cfg); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestParseConfigIPv6Endpoint(t *testing.T) {
	cfg := "[Interface]\nPrivateKey = x\nAddress = 10.0.0.2/32\n[Peer]\nPublicKey = y\nEndpoint = [2001:db8::1]:51820\nAllowedIPs = 0.0.0.0/0\n"
	c, err := ParseConfig(cfg)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if c.EndpointHost != "2001:db8::1" {
		t.Errorf("ipv6 endpoint host = %q, want 2001:db8::1", c.EndpointHost)
	}
}

func TestEnsureDNSInjectsWhenMissing(t *testing.T) {
	cfg := "[Interface]\nPrivateKey = x\nAddress = 10.0.0.2/32\nJc = 4\n\n[Peer]\nPublicKey = y\nEndpoint = 1.2.3.4:5\nAllowedIPs = 0.0.0.0/0\n"
	out := ensureDNS(cfg, "1.1.1.1")
	if !strings.Contains(out, "DNS = 1.1.1.1") {
		t.Fatalf("DNS not injected:\n%s", out)
	}
	// Must inject inside [Interface], before [Peer], and keep obfuscation.
	iface := strings.Index(out, "[Interface]")
	dns := strings.Index(out, "DNS = 1.1.1.1")
	peer := strings.Index(out, "[Peer]")
	if !(iface < dns && dns < peer) {
		t.Errorf("DNS injected in wrong place (iface=%d dns=%d peer=%d)", iface, dns, peer)
	}
	if !strings.Contains(out, "Jc = 4") {
		t.Error("obfuscation param dropped during DNS injection")
	}
}

func TestPinEndpoint(t *testing.T) {
	cfg := "[Interface]\nPrivateKey = x\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = y\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n"
	out := pinEndpoint(cfg, "203.0.113.9")
	if !strings.Contains(out, "Endpoint = 203.0.113.9:51820") {
		t.Fatalf("endpoint not pinned:\n%s", out)
	}
	if strings.Contains(out, "vpn.example.com") {
		t.Error("original hostname still present after pinning")
	}
	// Address in [Interface] must be untouched (only [Peer] Endpoint changes).
	if !strings.Contains(out, "Address = 10.0.0.2/32") {
		t.Error("pinEndpoint disturbed the [Interface] section")
	}
}

func TestPinEndpointIPv6(t *testing.T) {
	cfg := "[Peer]\nEndpoint = host.example:443\nAllowedIPs = ::/0\n"
	out := pinEndpoint(cfg, "2001:db8::1")
	if !strings.Contains(out, "Endpoint = [2001:db8::1]:443") {
		t.Fatalf("ipv6 endpoint not pinned with brackets:\n%s", out)
	}
}

func TestEnsureDNSLeavesExistingUntouched(t *testing.T) {
	if out := ensureDNS(goodConfig, "9.9.9.9"); out != goodConfig {
		t.Error("ensureDNS modified a config that already had DNS")
	}
	if strings.Contains(ensureDNS(goodConfig, "9.9.9.9"), "9.9.9.9") {
		t.Error("ensureDNS injected a fallback despite existing DNS")
	}
}
