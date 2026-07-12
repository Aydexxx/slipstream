package killswitch

import (
	"net/netip"

	"golang.org/x/sys/windows"
)

// Fixed GUIDs identify every WFP object Slipstream creates, so we can add and
// (crucially) delete them by key across process restarts without enumeration —
// which is what makes crash reconciliation deterministic.
var (
	providerKey = windows.GUID{Data1: 0x5117b2ee, Data2: 0x1111, Data3: 0x4b1d, Data4: [8]byte{0x9c, 0x0f, 0, 0, 0, 0, 0, 0x01}}
	sublayerKey = windows.GUID{Data1: 0x5117b2ee, Data2: 0x1111, Data3: 0x4b1d, Data4: [8]byte{0x9c, 0x0f, 0, 0, 0, 0, 0, 0x02}}

	keyBlockV4  = filterGUID(0x10)
	keyBlockV6  = filterGUID(0x11)
	keyLoopV4   = filterGUID(0x12)
	keyLoopV6   = filterGUID(0x13)
	keyDHCPv4   = filterGUID(0x14)
	keyEndpoint = filterGUID(0x15)
	keyTunV4    = filterGUID(0x16)
	keyTunV6    = filterGUID(0x17)
)

func filterGUID(last byte) windows.GUID {
	return windows.GUID{Data1: 0x5117b2ee, Data2: 0x1111, Data3: 0x4b1d, Data4: [8]byte{0x9c, 0x0f, 0, 0, 0, 0, 0, last}}
}

// allFilterKeys is every filter GUID we might create. Disarm/Reconcile delete
// all of them unconditionally (missing ones are ignored), so teardown never
// depends on knowing which were actually added.
func allFilterKeys() []windows.GUID {
	return []windows.GUID{keyBlockV4, keyBlockV6, keyLoopV4, keyLoopV6, keyDHCPv4, keyEndpoint, keyTunV4, keyTunV6}
}

// Permit filters outweigh the block; among terminating filters in our sublayer
// the highest-weight match wins, so any permitted flow (loopback, endpoint,
// DHCP, tunnel) beats the catch-all block.
const permitWeight = 15

// Params describe what the kill switch must let through besides the tunnel.
type Params struct {
	EndpointIP   netip.Addr // resolved VPS IP (v4 or v6) — the handshake must reach it
	EndpointPort uint16     // VPS UDP port
	AllowLANDHCP bool       // permit DHCP so the underlying link keeps its lease
}

// condSpec is one filter condition as plain data (WFP-struct-free, testable).
type condSpec struct {
	fieldKey  windows.GUID
	matchType uint32
	valueType uint32
	u64       uint64    // numeric conditions (u8/u16/u32/u64/flags)
	v6        *[16]byte // IPv6 address condition (FWP_BYTE_ARRAY16)
}

// filterSpec is one WFP filter as plain data.
type filterSpec struct {
	key        windows.GUID
	name       string
	layer      windows.GUID
	action     uint32 // actionBlock / actionPermit
	weight     uint8
	conditions []condSpec
}

func conditionU8(field windows.GUID, v uint8) condSpec {
	return condSpec{fieldKey: field, matchType: fwpMatchEqual, valueType: fwpUint8, u64: uint64(v)}
}
func conditionU16(field windows.GUID, v uint16) condSpec {
	return condSpec{fieldKey: field, matchType: fwpMatchEqual, valueType: fwpUint16, u64: uint64(v)}
}
func conditionU32(field windows.GUID, v uint32) condSpec {
	return condSpec{fieldKey: field, matchType: fwpMatchEqual, valueType: fwpUint32, u64: uint64(v)}
}
func conditionU64(field windows.GUID, v uint64) condSpec {
	return condSpec{fieldKey: field, matchType: fwpMatchEqual, valueType: fwpUint64, u64: v}
}
func conditionFlags(field windows.GUID, flag uint32) condSpec {
	return condSpec{fieldKey: field, matchType: fwpMatchFlagsAnySet, valueType: fwpUint32, u64: uint64(flag)}
}
func conditionV6Addr(field windows.GUID, a [16]byte) condSpec {
	arr := a
	return condSpec{fieldKey: field, matchType: fwpMatchEqual, valueType: fwpByteArray16, v6: &arr}
}

// baseFilterSpecs are the filters installed at Arm time: block everything
// outbound (v4+v6), then punch holes for loopback, the VPS endpoint, and DHCP.
// The tunnel permit is added later (once the adapter exists) so that during the
// connect window nothing but the handshake can leave the machine.
func baseFilterSpecs(p Params) []filterSpec {
	udp := conditionU8(condIPProtocol, protoUDP)

	specs := []filterSpec{
		{key: keyBlockV4, name: "Slipstream block-all outbound (v4)", layer: layerConnectV4, action: actionBlock, weight: 0},
		{key: keyBlockV6, name: "Slipstream block-all outbound (v6)", layer: layerConnectV6, action: actionBlock, weight: 0},
		{key: keyLoopV4, name: "Slipstream permit loopback (v4)", layer: layerConnectV4, action: actionPermit, weight: permitWeight,
			conditions: []condSpec{conditionFlags(condFlags, condFlagIsLoopback)}},
		{key: keyLoopV6, name: "Slipstream permit loopback (v6)", layer: layerConnectV6, action: actionPermit, weight: permitWeight,
			conditions: []condSpec{conditionFlags(condFlags, condFlagIsLoopback)}},
	}

	if p.AllowLANDHCP {
		specs = append(specs, filterSpec{
			key: keyDHCPv4, name: "Slipstream permit DHCP (v4)", layer: layerConnectV4, action: actionPermit, weight: permitWeight,
			conditions: []condSpec{udp, conditionU16(condIPRemotePort, 67)},
		})
	}

	// The single VPS endpoint IP:port — the only unencapsulated destination the
	// machine may reach, so the WireGuard handshake survives the block.
	epConds := []condSpec{udp, conditionU16(condIPRemotePort, p.EndpointPort)}
	if p.EndpointIP.Is4() {
		epConds = append(epConds, conditionU32(condIPRemoteAddress, v4ToUint32(p.EndpointIP)))
		specs = append(specs, filterSpec{key: keyEndpoint, name: "Slipstream permit VPS endpoint", layer: layerConnectV4, action: actionPermit, weight: permitWeight, conditions: epConds})
	} else {
		epConds = append(epConds, conditionV6Addr(condIPRemoteAddress, p.EndpointIP.As16()))
		specs = append(specs, filterSpec{key: keyEndpoint, name: "Slipstream permit VPS endpoint", layer: layerConnectV6, action: actionPermit, weight: permitWeight, conditions: epConds})
	}
	return specs
}

// tunnelFilterSpecs permit all traffic whose local interface is the tunnel
// adapter (both address families), added once the adapter's LUID is known.
func tunnelFilterSpecs(luid uint64) []filterSpec {
	return []filterSpec{
		{key: keyTunV4, name: "Slipstream permit tunnel (v4)", layer: layerConnectV4, action: actionPermit, weight: permitWeight,
			conditions: []condSpec{conditionU64(condIPLocalInterface, luid)}},
		{key: keyTunV6, name: "Slipstream permit tunnel (v6)", layer: layerConnectV6, action: actionPermit, weight: permitWeight,
			conditions: []condSpec{conditionU64(condIPLocalInterface, luid)}},
	}
}

// v4ToUint32 converts an IPv4 address to the host-byte-order UINT32 WFP expects
// for IP_REMOTE_ADDRESS (first octet in the most-significant byte).
func v4ToUint32(a netip.Addr) uint32 {
	b := a.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}
