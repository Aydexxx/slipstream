// Package dpapi wraps the Windows Data Protection API (DPAPI) so Slipstream can
// encrypt secrets at rest tied to the current Windows user account. It is used
// to store the imported Private Mode (AmneziaWG) config — which contains the
// tunnel's private key — under %LocalAppData% without ever writing the key in
// plaintext.
//
// Blobs are protected in *user* scope (only the same Windows user can decrypt)
// and bound to an app-specific entropy value, so a blob copied to another
// machine or another account is useless.
package dpapi

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	crypt32       = windows.NewLazySystemDLL("crypt32.dll")
	procProtect   = crypt32.NewProc("CryptProtectData")
	procUnprotect = crypt32.NewProc("CryptUnprotectData")
)

// CRYPTPROTECT_UI_FORBIDDEN — never show UI; fail instead if one would appear.
const cryptUIForbidden = 0x1

// entropy binds protected blobs to Slipstream's Private Mode store. Changing it
// invalidates every previously stored config (they'd have to be re-imported).
var entropy = []byte("Slipstream-PrivateMode-v1")

// dataBlob mirrors the Win32 DATA_BLOB (DWORD cbData; BYTE* pbData).
type dataBlob struct {
	cbData uint32
	pbData *byte
}

func toBlob(b []byte) dataBlob {
	if len(b) == 0 {
		return dataBlob{}
	}
	return dataBlob{cbData: uint32(len(b)), pbData: &b[0]}
}

// Protect encrypts plaintext with DPAPI (user scope + app entropy).
func Protect(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("dpapi: refusing to protect empty data")
	}
	return crypt(procProtect, plaintext)
}

// Unprotect decrypts a blob previously produced by Protect on this account.
func Unprotect(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, fmt.Errorf("dpapi: refusing to unprotect empty data")
	}
	return crypt(procUnprotect, ciphertext)
}

// crypt invokes CryptProtectData/CryptUnprotectData, copies the result into a
// Go-owned slice, and frees the Win32 buffer.
func crypt(proc *windows.LazyProc, in []byte) ([]byte, error) {
	inBlob := toBlob(in)
	entBlob := toBlob(entropy)
	var out dataBlob

	r, _, callErr := proc.Call(
		uintptr(unsafe.Pointer(&inBlob)),
		0, // szDataDescr
		uintptr(unsafe.Pointer(&entBlob)),
		0, // pvReserved
		0, // pPromptStruct
		cryptUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	// Keep the input buffers alive across the syscall.
	runtime.KeepAlive(in)
	runtime.KeepAlive(entropy)

	if r == 0 {
		return nil, fmt.Errorf("dpapi: operation failed: %w", callErr)
	}
	if out.pbData == nil {
		return nil, fmt.Errorf("dpapi: operation returned no data")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.pbData)))

	res := make([]byte, out.cbData)
	copy(res, unsafe.Slice(out.pbData, out.cbData))
	return res, nil
}
