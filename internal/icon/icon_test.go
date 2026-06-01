package icon

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

// TestRenderDimensions checks both forms come out at the requested size and that
// the tile form is opaque in the middle while the bare glyph is transparent in the
// corners (so it sits cleanly on any tray background).
func TestRenderDimensions(t *testing.T) {
	for _, s := range []int{16, 32, 256} {
		if g := Glyph(s); g.Bounds().Dx() != s || g.Bounds().Dy() != s {
			t.Errorf("Glyph(%d) size = %v", s, g.Bounds())
		}
		if a := App(s); a.Bounds().Dx() != s || a.Bounds().Dy() != s {
			t.Errorf("App(%d) size = %v", s, a.Bounds())
		}
	}

	g := Glyph(32)
	if _, _, _, alpha := g.At(0, 0).RGBA(); alpha != 0 {
		t.Errorf("glyph corner alpha = %d, want 0 (transparent)", alpha)
	}
	a := App(64)
	if _, _, _, alpha := a.At(32, 32).RGBA(); alpha != 0xffff {
		t.Errorf("app-icon centre alpha = %d, want opaque", alpha)
	}
}

// TestICOStructure builds a multi-size icon and parses it back, checking the
// ICONDIR header, every directory entry, and each embedded DIB header.
func TestICOStructure(t *testing.T) {
	sizes := []int{16, 32}
	imgs := make([]*image.RGBA, len(sizes))
	for i, s := range sizes {
		imgs[i] = image.NewRGBA(image.Rect(0, 0, s, s))
	}

	data := ICO(imgs)
	le := binary.LittleEndian

	if got := le.Uint16(data[0:2]); got != 0 {
		t.Errorf("reserved = %d, want 0", got)
	}
	if got := le.Uint16(data[2:4]); got != 1 {
		t.Errorf("type = %d, want 1 (icon)", got)
	}
	if got := int(le.Uint16(data[4:6])); got != len(sizes) {
		t.Fatalf("image count = %d, want %d", got, len(sizes))
	}

	for i, s := range sizes {
		entry := data[6+16*i : 6+16*(i+1)]
		if int(entry[0]) != s%256 || int(entry[1]) != s%256 {
			t.Errorf("entry %d dimensions = %dx%d, want %dx%d", i, entry[0], entry[1], s, s)
		}
		if bpp := le.Uint16(entry[6:8]); bpp != 32 {
			t.Errorf("entry %d bpp = %d, want 32", i, bpp)
		}
		size := int(le.Uint32(entry[8:12]))
		offset := int(le.Uint32(entry[12:16]))
		if offset+size > len(data) {
			t.Fatalf("entry %d points past end: offset=%d size=%d total=%d", i, offset, size, len(data))
		}
		maskRow := ((s + 31) / 32) * 4
		if want := 40 + s*s*4 + maskRow*s; size != want {
			t.Errorf("entry %d DIB size = %d, want %d", i, size, want)
		}
	}
}

// TestDIBUnpremultiplies verifies a half-transparent pixel is written with straight
// (non-premultiplied) colour, so anti-aliased edges keep their hue in Windows DIBs.
func TestDIBUnpremultiplies(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{0x80, 0x40, 0x20, 0x80}) // premultiplied (alpha 0x80)
	dib := DIB(img)
	// BGRA pixel sits right after the 40-byte header.
	b, g, r, a := dib[40], dib[41], dib[42], dib[43]
	if a != 0x80 {
		t.Fatalf("alpha = %#x, want 0x80", a)
	}
	// 0x80 premultiplied with alpha 0x80 → ~0xff straight.
	if r != 0xff || g != 0x7f && g != 0x80 || b != 0x3f && b != 0x40 {
		t.Errorf("straight BGRA = %#x %#x %#x %#x, want roughly ff 7f 3f 80 unpremultiplied", b, g, r, a)
	}
}

// TestSYSOStructure encodes a real icon resource and parses the COFF back with
// debug/pe, confirming the section layout and relocations the Windows linker needs.
func TestSYSOStructure(t *testing.T) {
	imgs := []*image.RGBA{App(16), App(32), App(48), App(256)}
	syso, err := SYSO(MachineAMD64, imgs)
	if err != nil {
		t.Fatal(err)
	}

	f, err := pe.NewFile(bytes.NewReader(syso))
	if err != nil {
		t.Fatalf("debug/pe could not parse .syso: %v", err)
	}
	if f.Machine != MachineAMD64 {
		t.Errorf("machine = %#x, want %#x", f.Machine, MachineAMD64)
	}
	if len(f.Sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(f.Sections))
	}
	if f.Sections[0].Name != ".rsrc$01" || f.Sections[1].Name != ".rsrc$02" {
		t.Errorf("section names = %q,%q", f.Sections[0].Name, f.Sections[1].Name)
	}
	// One relocation per resource data entry (icons + group directory).
	if got, want := len(f.Sections[0].Relocs), len(imgs)+1; got != want {
		t.Errorf("relocations = %d, want %d", got, want)
	}

	if _, err := SYSO(0x1234, imgs); err == nil {
		t.Error("SYSO accepted an unsupported machine type")
	}
}
