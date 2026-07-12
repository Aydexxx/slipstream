package killswitch

import (
	"testing"
	"unsafe"
)

// These assertions pin the Go structs to the documented 64-bit WFP C ABI. A
// wrong size or field offset (the classic union/alignment mistake) fails here
// on any Windows machine — no Administrator, no live WFP engine needed — so the
// hand-rolled binding can't silently pass garbage to fwpuclnt.dll.
func TestWFPStructLayout(t *testing.T) {
	check := func(name string, got, want uintptr) {
		if got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}

	check("sizeof fwpValue0", unsafe.Sizeof(fwpValue0{}), 16)
	check("offsetof fwpValue0.value", unsafe.Offsetof(fwpValue0{}.value), 8)

	check("sizeof fwpConditionValue0", unsafe.Sizeof(fwpConditionValue0{}), 16)

	check("sizeof fwpmDisplayData0", unsafe.Sizeof(fwpmDisplayData0{}), 16)
	check("sizeof fwpByteBlob", unsafe.Sizeof(fwpByteBlob{}), 16)
	check("offsetof fwpByteBlob.data", unsafe.Offsetof(fwpByteBlob{}.data), 8)

	check("sizeof fwpmFilterCondition0", unsafe.Sizeof(fwpmFilterCondition0{}), 40)
	check("offsetof cond.matchType", unsafe.Offsetof(fwpmFilterCondition0{}.matchType), 16)
	check("offsetof cond.conditionValue", unsafe.Offsetof(fwpmFilterCondition0{}.conditionValue), 24)

	check("sizeof fwpmAction0", unsafe.Sizeof(fwpmAction0{}), 20)
	check("offsetof action.filterType", unsafe.Offsetof(fwpmAction0{}.filterType), 4)

	check("sizeof fwpmProvider0", unsafe.Sizeof(fwpmProvider0{}), 64)

	// FWPM_FILTER0: the alignment-sensitive struct. Offsets from the C layout.
	f := fwpmFilter0{}
	check("sizeof fwpmFilter0", unsafe.Sizeof(f), 200)
	check("offsetof filter.layerKey", unsafe.Offsetof(f.layerKey), 64)
	check("offsetof filter.subLayerKey", unsafe.Offsetof(f.subLayerKey), 80)
	check("offsetof filter.weight", unsafe.Offsetof(f.weight), 96)
	check("offsetof filter.numFilterConditions", unsafe.Offsetof(f.numFilterConditions), 112)
	check("offsetof filter.filterCondition", unsafe.Offsetof(f.filterCondition), 120)
	check("offsetof filter.action", unsafe.Offsetof(f.action), 128)
	check("offsetof filter.context", unsafe.Offsetof(f.context), 152)
	check("offsetof filter.filterID", unsafe.Offsetof(f.filterID), 176)
	check("offsetof filter.effectiveWeight", unsafe.Offsetof(f.effectiveWeight), 184)
}
