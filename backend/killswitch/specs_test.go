package killswitch

import (
	"net/netip"
	"testing"
)

func findFilter(specs []filterSpec, key any) *filterSpec {
	for i := range specs {
		if specs[i].key == key {
			return &specs[i]
		}
	}
	return nil
}

func hasCondition(f *filterSpec, field any) *condSpec {
	for i := range f.conditions {
		if f.conditions[i].fieldKey == field {
			return &f.conditions[i]
		}
	}
	return nil
}

func TestBaseFilterSpecsV4Endpoint(t *testing.T) {
	p := Params{EndpointIP: netip.MustParseAddr("203.0.113.9"), EndpointPort: 51820, AllowLANDHCP: true}
	specs := baseFilterSpecs(p)

	// Must always block both families.
	for _, k := range []any{keyBlockV4, keyBlockV6} {
		f := findFilter(specs, k)
		if f == nil {
			t.Fatalf("missing block filter %v", k)
		}
		if f.action != actionBlock || f.weight != 0 || len(f.conditions) != 0 {
			t.Errorf("block filter %v malformed: %+v", k, f)
		}
	}

	// Loopback permitted on both families.
	if findFilter(specs, keyLoopV4) == nil || findFilter(specs, keyLoopV6) == nil {
		t.Error("loopback permit missing")
	}

	// DHCP permitted (requested).
	if findFilter(specs, keyDHCPv4) == nil {
		t.Error("DHCP permit missing when AllowLANDHCP=true")
	}

	// Endpoint permit is on the v4 layer with addr+port+proto conditions.
	ep := findFilter(specs, keyEndpoint)
	if ep == nil {
		t.Fatal("endpoint permit missing")
	}
	if ep.layer != layerConnectV4 || ep.action != actionPermit {
		t.Errorf("endpoint filter wrong layer/action: %+v", ep)
	}
	addr := hasCondition(ep, condIPRemoteAddress)
	if addr == nil || addr.valueType != fwpUint32 || addr.u64 != uint64(v4ToUint32(p.EndpointIP)) {
		t.Errorf("endpoint address condition wrong: %+v", addr)
	}
	if port := hasCondition(ep, condIPRemotePort); port == nil || port.u64 != 51820 {
		t.Errorf("endpoint port condition wrong: %+v", port)
	}
	if proto := hasCondition(ep, condIPProtocol); proto == nil || proto.u64 != protoUDP {
		t.Errorf("endpoint protocol condition wrong: %+v", proto)
	}
}

func TestBaseFilterSpecsV6Endpoint(t *testing.T) {
	p := Params{EndpointIP: netip.MustParseAddr("2001:db8::1"), EndpointPort: 443}
	specs := baseFilterSpecs(p)

	ep := findFilter(specs, keyEndpoint)
	if ep == nil || ep.layer != layerConnectV6 {
		t.Fatalf("v6 endpoint permit should be on v6 layer: %+v", ep)
	}
	addr := hasCondition(ep, condIPRemoteAddress)
	if addr == nil || addr.valueType != fwpByteArray16 || addr.v6 == nil {
		t.Fatalf("v6 endpoint address condition wrong: %+v", addr)
	}
	want := p.EndpointIP.As16()
	if *addr.v6 != want {
		t.Errorf("v6 endpoint bytes = %v, want %v", *addr.v6, want)
	}

	// DHCP not requested → absent.
	if findFilter(specs, keyDHCPv4) != nil {
		t.Error("DHCP permit present despite AllowLANDHCP=false")
	}
}

func TestTunnelFilterSpecs(t *testing.T) {
	specs := tunnelFilterSpecs(0xdeadbeef)
	if len(specs) != 2 {
		t.Fatalf("want 2 tunnel filters, got %d", len(specs))
	}
	for _, s := range specs {
		if s.action != actionPermit {
			t.Errorf("tunnel filter not permit: %+v", s)
		}
		c := hasCondition(&s, condIPLocalInterface)
		if c == nil || c.valueType != fwpUint64 || c.u64 != 0xdeadbeef {
			t.Errorf("tunnel filter local-interface condition wrong: %+v", c)
		}
	}
}

func TestV4ToUint32(t *testing.T) {
	// 203.0.113.9 => 0xCB007109
	if got := v4ToUint32(netip.MustParseAddr("203.0.113.9")); got != 0xCB007109 {
		t.Errorf("v4ToUint32 = 0x%x, want 0xCB007109", got)
	}
	if got := v4ToUint32(netip.MustParseAddr("1.2.3.4")); got != 0x01020304 {
		t.Errorf("v4ToUint32 = 0x%x, want 0x01020304", got)
	}
}

func TestAllFilterKeysCoversEverything(t *testing.T) {
	keys := allFilterKeys()
	if len(keys) != 8 {
		t.Fatalf("allFilterKeys should list every filter GUID, got %d", len(keys))
	}
	// No duplicates (each GUID distinct).
	seen := map[windowsGUIDKey]bool{}
	for _, k := range keys {
		gk := windowsGUIDKey{k.Data1, k.Data2, k.Data3, k.Data4}
		if seen[gk] {
			t.Errorf("duplicate filter key %v", k)
		}
		seen[gk] = true
	}
}

type windowsGUIDKey struct {
	a uint32
	b uint16
	c uint16
	d [8]byte
}
