package killswitch

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This file is the low-level Windows Filtering Platform (WFP) binding: struct
// layouts mirroring fwpmtypes.h/fwptypes.h, the fwpuclnt.dll entry points, and
// helpers to open an engine, run a transaction, and add/delete our objects.
//
// The struct layouts are the highest-risk part (WFP structs contain unions and
// alignment padding). wfp_layout_test.go asserts their sizes/offsets against the
// documented 64-bit C layout, so a mistake is caught by `go test` on any
// Windows box — no Administrator or live tunnel required.

// --- FWP enum constants (fwptypes.h) ---
const (
	fwpUint8       = 1
	fwpUint16      = 2
	fwpUint32      = 3
	fwpUint64      = 4
	fwpByteArray16 = 11

	fwpMatchEqual       = 0
	fwpMatchFlagsAnySet = 7

	// FWP_ACTION_FLAG_TERMINATING | type
	actionBlock  = 0x1001 // FWP_ACTION_BLOCK
	actionPermit = 0x1002 // FWP_ACTION_PERMIT

	condFlagIsLoopback = 0x00000001 // FWP_CONDITION_FLAG_IS_LOOPBACK
	protoUDP           = 17

	rpcCAuthnWinNT = 10 // RPC_C_AUTHN_WINNT

	// FWP error codes (winerror.h, 0x8032xxxx) we tolerate.
	fwpErrFilterNotFound   = 0x80320003
	fwpErrProviderNotFound = 0x80320005
	fwpErrSublayerNotFound = 0x80320007
	fwpErrAlreadyExists    = 0x80320009
)

// --- Well-known WFP GUIDs (fwpmu.h) ---
var (
	layerConnectV4 = windows.GUID{Data1: 0xc38d57d1, Data2: 0x05a7, Data3: 0x4c33, Data4: [8]byte{0x90, 0x4f, 0x7f, 0xbc, 0xee, 0xe6, 0x0e, 0x82}}
	layerConnectV6 = windows.GUID{Data1: 0x4a72393b, Data2: 0x319f, Data3: 0x44bc, Data4: [8]byte{0x84, 0xc3, 0xba, 0x54, 0xdc, 0xb3, 0xb6, 0xb4}}

	condIPRemoteAddress  = windows.GUID{Data1: 0xb235ae9a, Data2: 0x1d64, Data3: 0x49b8, Data4: [8]byte{0xa4, 0x4c, 0x5f, 0xf3, 0xd9, 0x09, 0x50, 0x45}}
	condIPRemotePort     = windows.GUID{Data1: 0xc35a604d, Data2: 0xd22b, Data3: 0x48b1, Data4: [8]byte{0xb6, 0x21, 0xa4, 0xcd, 0x7e, 0xdc, 0x1c, 0xb6}}
	condIPProtocol       = windows.GUID{Data1: 0x3971ef2b, Data2: 0x623e, Data3: 0x4f9a, Data4: [8]byte{0x8c, 0xb1, 0x6e, 0x79, 0xb8, 0x06, 0xb9, 0xa7}}
	condIPLocalInterface = windows.GUID{Data1: 0x4cd62a49, Data2: 0x59c3, Data3: 0x4969, Data4: [8]byte{0xb7, 0xf3, 0xbd, 0xa5, 0xd3, 0x28, 0x90, 0xa4}}
	condFlags            = windows.GUID{Data1: 0x632ce23b, Data2: 0x5167, Data3: 0x435c, Data4: [8]byte{0x86, 0xd7, 0xe9, 0x03, 0x68, 0x4a, 0xa8, 0x0c}}
)

// --- Struct layouts (must match the C ABI; verified by wfp_layout_test.go) ---

type fwpValue0 struct {
	valueType uint32
	// 4 bytes padding here on 64-bit (Go inserts it: next field is 8-aligned)
	value uintptr
}

type fwpConditionValue0 struct {
	valueType uint32
	value     uintptr
}

type fwpmDisplayData0 struct {
	name        *uint16
	description *uint16
}

type fwpByteBlob struct {
	size uint32
	data *uint8
}

type fwpmFilterCondition0 struct {
	fieldKey       windows.GUID
	matchType      uint32
	conditionValue fwpConditionValue0
}

type fwpmAction0 struct {
	actionType uint32
	filterType windows.GUID // union { filterType; calloutKey } — left zero
}

// contextUnion models the FWPM_FILTER0 union { UINT64 rawContext; GUID
// providerContextKey }. Modeled as two uint64 so it carries the union's 8-byte
// alignment (a [16]byte would mis-align the following fields).
type contextUnion struct {
	lo uint64
	hi uint64
}

type fwpmFilter0 struct {
	filterKey           windows.GUID
	displayData         fwpmDisplayData0
	flags               uint32
	providerKey         *windows.GUID
	providerData        fwpByteBlob
	layerKey            windows.GUID
	subLayerKey         windows.GUID
	weight              fwpValue0
	numFilterConditions uint32
	filterCondition     *fwpmFilterCondition0
	action              fwpmAction0
	context             contextUnion
	reserved            *windows.GUID
	filterID            uint64
	effectiveWeight     fwpValue0
}

type fwpmProvider0 struct {
	providerKey  windows.GUID
	displayData  fwpmDisplayData0
	flags        uint32
	providerData fwpByteBlob
	serviceName  *uint16
}

type fwpmSublayer0 struct {
	subLayerKey  windows.GUID
	displayData  fwpmDisplayData0
	flags        uint32
	providerKey  *windows.GUID
	providerData fwpByteBlob
	weight       uint16
}

// --- fwpuclnt.dll entry points ---
var (
	fwpuclnt = windows.NewLazySystemDLL("fwpuclnt.dll")

	procEngineOpen          = fwpuclnt.NewProc("FwpmEngineOpen0")
	procEngineClose         = fwpuclnt.NewProc("FwpmEngineClose0")
	procTransactionBegin    = fwpuclnt.NewProc("FwpmTransactionBegin0")
	procTransactionCommit   = fwpuclnt.NewProc("FwpmTransactionCommit0")
	procTransactionAbort    = fwpuclnt.NewProc("FwpmTransactionAbort0")
	procProviderAdd         = fwpuclnt.NewProc("FwpmProviderAdd0")
	procProviderDeleteByKey = fwpuclnt.NewProc("FwpmProviderDeleteByKey0")
	procSubLayerAdd         = fwpuclnt.NewProc("FwpmSubLayerAdd0")
	procSubLayerDeleteByKey = fwpuclnt.NewProc("FwpmSubLayerDeleteByKey0")
	procFilterAdd           = fwpuclnt.NewProc("FwpmFilterAdd0")
	procFilterDeleteByKey   = fwpuclnt.NewProc("FwpmFilterDeleteByKey0")
)

// engine is an open WFP engine handle.
type engine windows.Handle

// openEngine opens a non-dynamic WFP session (session=NULL). Non-dynamic means
// the objects we add persist after this handle/process closes — that is what
// makes the kill switch fail closed on a crash.
func openEngine() (engine, error) {
	var h windows.Handle
	r, _, _ := procEngineOpen.Call(0, rpcCAuthnWinNT, 0, 0, uintptr(unsafe.Pointer(&h)))
	if r != 0 {
		return 0, fmt.Errorf("FwpmEngineOpen0: 0x%x", r)
	}
	return engine(h), nil
}

func (e engine) close() { _, _, _ = procEngineClose.Call(uintptr(e)) }

func (e engine) begin() error {
	if r, _, _ := procTransactionBegin.Call(uintptr(e), 0); r != 0 {
		return fmt.Errorf("FwpmTransactionBegin0: 0x%x", r)
	}
	return nil
}

func (e engine) commit() error {
	if r, _, _ := procTransactionCommit.Call(uintptr(e)); r != 0 {
		return fmt.Errorf("FwpmTransactionCommit0: 0x%x", r)
	}
	return nil
}

func (e engine) abort() { _, _, _ = procTransactionAbort.Call(uintptr(e)) }

// ensureProvider/ensureSublayer add our provider and sublayer, tolerating
// "already exists" so Arm is idempotent.
func (e engine) ensureProvider() error {
	name, _ := windows.UTF16PtrFromString("Slipstream Kill Switch")
	p := fwpmProvider0{providerKey: providerKey, displayData: fwpmDisplayData0{name: name}}
	r, _, _ := procProviderAdd.Call(uintptr(e), uintptr(unsafe.Pointer(&p)), 0)
	runtime.KeepAlive(name)
	if r != 0 && r != fwpErrAlreadyExists {
		return fmt.Errorf("FwpmProviderAdd0: 0x%x", r)
	}
	return nil
}

func (e engine) ensureSublayer() error {
	name, _ := windows.UTF16PtrFromString("Slipstream Kill Switch")
	pk := providerKey
	s := fwpmSublayer0{subLayerKey: sublayerKey, displayData: fwpmDisplayData0{name: name}, providerKey: &pk, weight: 0xFFFF}
	r, _, _ := procSubLayerAdd.Call(uintptr(e), uintptr(unsafe.Pointer(&s)), 0)
	runtime.KeepAlive(name)
	runtime.KeepAlive(&pk)
	if r != 0 && r != fwpErrAlreadyExists {
		return fmt.Errorf("FwpmSubLayerAdd0: 0x%x", r)
	}
	return nil
}

// addFilter builds the WFP filter for spec and installs it.
func (e engine) addFilter(spec filterSpec) error {
	conds := make([]fwpmFilterCondition0, len(spec.conditions))
	for i, c := range spec.conditions {
		fc := fwpmFilterCondition0{fieldKey: c.fieldKey, matchType: c.matchType}
		fc.conditionValue.valueType = c.valueType
		if c.v6 != nil {
			fc.conditionValue.value = uintptr(unsafe.Pointer(c.v6))
		} else {
			fc.conditionValue.value = uintptr(c.u64)
		}
		conds[i] = fc
	}

	name, _ := windows.UTF16PtrFromString(spec.name)
	pk := providerKey
	f := fwpmFilter0{
		filterKey:   spec.key,
		displayData: fwpmDisplayData0{name: name},
		providerKey: &pk,
		layerKey:    spec.layer,
		subLayerKey: sublayerKey,
		weight:      fwpValue0{valueType: fwpUint8, value: uintptr(spec.weight)},
		action:      fwpmAction0{actionType: spec.action},
	}
	if len(conds) > 0 {
		f.numFilterConditions = uint32(len(conds))
		f.filterCondition = &conds[0]
	}

	var id uint64
	r, _, _ := procFilterAdd.Call(uintptr(e), uintptr(unsafe.Pointer(&f)), 0, uintptr(unsafe.Pointer(&id)))
	// Keep everything the filter points at alive across the syscall.
	runtime.KeepAlive(conds)
	runtime.KeepAlive(spec.conditions)
	runtime.KeepAlive(name)
	runtime.KeepAlive(&pk)
	if r != 0 {
		return fmt.Errorf("FwpmFilterAdd0(%s): 0x%x", spec.name, r)
	}
	return nil
}

// replaceFilter deletes any existing filter with the same key, then adds it —
// so Arm can be re-run without ALREADY_EXISTS errors.
func (e engine) replaceFilter(spec filterSpec) error {
	e.deleteFilter(spec.key)
	return e.addFilter(spec)
}

func (e engine) deleteFilter(key windows.GUID) {
	k := key
	r, _, _ := procFilterDeleteByKey.Call(uintptr(e), uintptr(unsafe.Pointer(&k)))
	runtime.KeepAlive(&k)
	if r != 0 && r != fwpErrFilterNotFound {
		// Deletion is best-effort during teardown; surface nothing fatal.
		_ = r
	}
}

// removeAll deletes every Slipstream filter, then the sublayer and provider,
// inside one transaction. Missing objects are ignored, so it is safe to call
// when nothing is installed (clean disarm and crash reconciliation share it).
func (e engine) removeAll() error {
	if err := e.begin(); err != nil {
		return err
	}
	for _, k := range allFilterKeys() {
		e.deleteFilter(k)
	}
	sk := sublayerKey
	if r, _, _ := procSubLayerDeleteByKey.Call(uintptr(e), uintptr(unsafe.Pointer(&sk))); r != 0 && r != fwpErrSublayerNotFound {
		_ = r
	}
	runtime.KeepAlive(&sk)
	pk := providerKey
	if r, _, _ := procProviderDeleteByKey.Call(uintptr(e), uintptr(unsafe.Pointer(&pk))); r != 0 && r != fwpErrProviderNotFound {
		_ = r
	}
	runtime.KeepAlive(&pk)
	return e.commit()
}
