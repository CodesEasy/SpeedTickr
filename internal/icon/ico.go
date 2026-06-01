package icon

import (
	"bytes"
	"encoding/binary"
	"image"
)

// ICO packs RGBA images into a classic .ico using 32-bit BMP/DIB entries — the
// format Windows' LoadImage accepts most reliably for tray icons. The layout is
// platform-independent, so it is unit tested directly (see ico_test.go).
func ICO(imgs []*image.RGBA) []byte {
	var buf bytes.Buffer
	le := binary.LittleEndian

	// ICONDIR header.
	_ = binary.Write(&buf, le, uint16(0))         // reserved
	_ = binary.Write(&buf, le, uint16(1))         // type: icon
	_ = binary.Write(&buf, le, uint16(len(imgs))) // image count

	dibs := make([][]byte, len(imgs))
	for i, im := range imgs {
		dibs[i] = DIB(im)
	}

	offset := 6 + 16*len(imgs) // header + one ICONDIRENTRY per image
	for i, im := range imgs {
		b := im.Bounds()
		buf.WriteByte(byte(b.Dx() % 256))                // width (0 means 256)
		buf.WriteByte(byte(b.Dy() % 256))                // height
		buf.WriteByte(0)                                 // palette size
		buf.WriteByte(0)                                 // reserved
		_ = binary.Write(&buf, le, uint16(1))            // colour planes
		_ = binary.Write(&buf, le, uint16(32))           // bits per pixel
		_ = binary.Write(&buf, le, uint32(len(dibs[i]))) // bytes of data
		_ = binary.Write(&buf, le, uint32(offset))       // data offset
		offset += len(dibs[i])
	}
	for _, d := range dibs {
		buf.Write(d)
	}
	return buf.Bytes()
}

// DIB encodes one image as a bottom-up 32-bit BITMAPINFOHEADER device-independent
// bitmap followed by the 1-bpp AND mask the icon format requires (left empty; the
// alpha channel carries transparency). This is exactly the per-image payload that an
// .ico entry and a Windows RT_ICON resource both expect.
//
// image.RGBA stores alpha-premultiplied colour; Windows DIBs use straight
// (non-premultiplied) alpha, so each pixel is divided back out — without this,
// anti-aliased edges would carry a dark halo.
func DIB(im *image.RGBA) []byte {
	b := im.Bounds()
	w, h := b.Dx(), b.Dy()
	var buf bytes.Buffer
	le := binary.LittleEndian

	_ = binary.Write(&buf, le, uint32(40)) // biSize
	_ = binary.Write(&buf, le, int32(w))   // biWidth
	_ = binary.Write(&buf, le, int32(2*h)) // biHeight: XOR image + AND mask
	_ = binary.Write(&buf, le, uint16(1))  // biPlanes
	_ = binary.Write(&buf, le, uint16(32)) // biBitCount
	_ = binary.Write(&buf, le, uint32(0))  // biCompression: BI_RGB
	_ = binary.Write(&buf, le, uint32(0))  // biSizeImage
	_ = binary.Write(&buf, le, int32(0))   // biXPelsPerMeter
	_ = binary.Write(&buf, le, int32(0))   // biYPelsPerMeter
	_ = binary.Write(&buf, le, uint32(0))  // biClrUsed
	_ = binary.Write(&buf, le, uint32(0))  // biClrImportant

	// XOR colour data, bottom-up rows, straight-alpha BGRA order.
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			o := im.PixOffset(b.Min.X+x, b.Min.Y+y)
			r, g, bl, a := im.Pix[o], im.Pix[o+1], im.Pix[o+2], im.Pix[o+3]
			r, g, bl = unpremul(r, a), unpremul(g, a), unpremul(bl, a)
			buf.WriteByte(bl)
			buf.WriteByte(g)
			buf.WriteByte(r)
			buf.WriteByte(a)
		}
	}

	// AND mask: 1 bit per pixel, rows padded to 4 bytes, all zero (use alpha).
	rowBytes := ((w + 31) / 32) * 4
	buf.Write(make([]byte, rowBytes*h))

	return buf.Bytes()
}

// unpremul recovers a straight-alpha channel value from a premultiplied one.
func unpremul(c, a uint8) uint8 {
	if a == 0 {
		return 0
	}
	v := int(c) * 255 / int(a)
	if v > 255 {
		v = 255
	}
	return uint8(v)
}
