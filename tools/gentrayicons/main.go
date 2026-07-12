//go:build ignore

// gentrayicons produces the placeholder tray-icon .ico files for
// backend/tray/icons. Run once via `go run tools/gentrayicons/main.go` from
// the repo root; the output is committed, this isn't run at build time.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const size = 32

type iconSpec struct {
	name string
	hex  string // "RRGGBB"
}

var icons = []iconSpec{
	{"off", "71717A"},     // neutral gray
	{"fast", "22C55E"},    // success green
	{"private", "8B7CF6"}, // accent violet
	{"alert", "EF4444"},   // danger red
}

func main() {
	outDir := filepath.Join("backend", "tray", "icons")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Println("mkdir:", err)
		os.Exit(1)
	}

	for _, ic := range icons {
		c, err := parseHexColor(ic.hex)
		if err != nil {
			fmt.Println("color:", err)
			os.Exit(1)
		}
		img := renderDot(c)

		var pngBuf bytes.Buffer
		if err := png.Encode(&pngBuf, img); err != nil {
			fmt.Println("png encode:", err)
			os.Exit(1)
		}

		icoBuf, err := wrapICO(pngBuf.Bytes(), size, size)
		if err != nil {
			fmt.Println("ico wrap:", err)
			os.Exit(1)
		}

		outPath := filepath.Join(outDir, ic.name+".ico")
		if err := os.WriteFile(outPath, icoBuf, 0o644); err != nil {
			fmt.Println("write:", err)
			os.Exit(1)
		}
		fmt.Println("wrote", outPath)
	}
}

// renderDot draws a filled, softly anti-aliased circle on a transparent
// background — a plain colored dot, matching the app's own StatusDot.
func renderDot(c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(size)/2, float64(size)/2
	radius := float64(size)/2 - 2 // small margin so it doesn't clip at 32px
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			dist := math.Sqrt(dx*dx + dy*dy)
			var alpha float64
			switch {
			case dist <= radius-1:
				alpha = 1
			case dist <= radius+1:
				alpha = (radius + 1 - dist) / 2 // 1px anti-aliased edge
			default:
				alpha = 0
			}
			if alpha <= 0 {
				continue
			}
			img.Set(x, y, color.RGBA{
				R: c.R, G: c.G, B: c.B,
				A: uint8(alpha * float64(c.A)),
			})
		}
	}
	return img
}

func parseHexColor(hex string) (color.RGBA, error) {
	var r, g, b uint8
	if _, err := fmt.Sscanf(hex, "%02X%02X%02X", &r, &g, &b); err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{R: r, G: g, B: b, A: 255}, nil
}

// wrapICO wraps a single PNG image in a minimal one-entry ICO container.
// Modern Windows (Vista+) accepts PNG-compressed image data directly inside
// an ICO, avoiding a hand-rolled BMP/AND-mask encoder.
func wrapICO(pngData []byte, w, h int) ([]byte, error) {
	var buf bytes.Buffer

	// ICONDIR
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // type = icon
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // count

	widthByte, heightByte := byte(w), byte(h)
	if w >= 256 {
		widthByte = 0
	}
	if h >= 256 {
		heightByte = 0
	}

	// ICONDIRENTRY
	buf.WriteByte(widthByte)
	buf.WriteByte(heightByte)
	buf.WriteByte(0)                                        // color count
	buf.WriteByte(0)                                        // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))       // planes
	binary.Write(&buf, binary.LittleEndian, uint16(32))      // bit count
	binary.Write(&buf, binary.LittleEndian, uint32(len(pngData))) // bytes in resource
	binary.Write(&buf, binary.LittleEndian, uint32(6+16))    // image offset

	buf.Write(pngData)
	return buf.Bytes(), nil
}
