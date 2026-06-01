// Package icon draws the SpeedTickr app mark and encodes it for each platform.
//
// The mark is two arrows — a green download arrow beside a blue upload arrow,
// mirroring the "↓ … ↑" text the app shows on the taskbar/menu bar. Everything is
// drawn procedurally (no asset files) so a single binary stays self-contained, and
// it is rendered with supersampled anti-aliasing so edges stay smooth at any size.
//
// Two forms are produced from the same arrows:
//
//   - Glyph: transparent background, for the system-tray / menu-bar icon, so it sits
//     cleanly on light or dark trays.
//   - App: arrows on a rounded dark tile, for the Windows executable icon (Explorer /
//     taskbar) and the project logo.
package icon

import (
	"image"
	"image/color"
	"math"
)

// Brand colours.
var (
	downGreen = color.RGBA{0x2f, 0xd1, 0x73, 0xff} // download
	upBlue    = color.RGBA{0x3b, 0x86, 0xf6, 0xff} // upload
	tileTop   = color.RGBA{0x27, 0x33, 0x46, 0xff} // tile gradient (top-left)
	tileBot   = color.RGBA{0x10, 0x18, 0x28, 0xff} // tile gradient (bottom-right)
)

// Arrow layout within the unit square. App keeps the arrows inset so they sit
// nicely on the tile; the bare Glyph enlarges them about the centre (glyphScale) so
// the tray icon carries the same visual weight as the icons beside it — the tile
// gives App that weight already, the lone arrows need it.
var (
	arrowDown = box{0.18, 0.27, 0.50, 0.73}
	arrowUp   = box{0.50, 0.27, 0.82, 0.73}
)

const glyphScale = 1.4

// Glyph renders the transparent two-arrow mark at size×size pixels — the system-tray
// and menu-bar icon, sized to fill the canvas like neighbouring tray icons.
func Glyph(size int) *image.RGBA {
	down, up := scaleBox(arrowDown, glyphScale), scaleBox(arrowUp, glyphScale)
	return render(size, func(x, y float64) pcol { return arrows(x, y, down, up, pcol{}) })
}

// App renders the mark on a rounded dark tile at size×size pixels — the form used
// for the executable icon and the logo image.
func App(size int) *image.RGBA {
	return render(size, func(x, y float64) pcol { return arrows(x, y, arrowDown, arrowUp, tile(x, y)) })
}

// arrows composites the download and upload arrows over bg. Coordinates are in the
// unit square so the layout is resolution-independent.
func arrows(x, y float64, down, up box, bg pcol) pcol {
	out := bg
	if insideArrow(x, y, down, true) {
		out = over(out, premul(downGreen))
	}
	if insideArrow(x, y, up, false) {
		out = over(out, premul(upBlue))
	}
	return out
}

// scaleBox enlarges b about the centre of the unit square by factor f.
func scaleBox(b box, f float64) box {
	const c = 0.5
	return box{c + (b.x0-c)*f, c + (b.y0-c)*f, c + (b.x1-c)*f, c + (b.y1-c)*f}
}

// tile is the rounded-rectangle background with a diagonal slate gradient.
func tile(u, v float64) pcol {
	const margin, radius = 0.05, 0.225
	if !insideRoundedRect(u, v, margin, margin, 1-margin, 1-margin, radius) {
		return pcol{}
	}
	t := (u + v) / 2
	lerp := func(a, b uint8) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }
	return premul(color.RGBA{
		lerp(tileTop.R, tileBot.R),
		lerp(tileTop.G, tileBot.G),
		lerp(tileTop.B, tileBot.B),
		0xff,
	})
}

// box is an axis-aligned rectangle in unit-square coordinates.
type box struct{ x0, y0, x1, y1 float64 }

// insideArrow reports whether (px,py) lies within a solid arrow (a stem plus a
// triangular head) bounded by b, pointing down or up.
func insideArrow(px, py float64, b box, down bool) bool {
	w, h := b.x1-b.x0, b.y1-b.y0
	cx := (b.x0 + b.x1) / 2
	stemHalf := 0.155 * w
	headHalf := 0.42 * w
	headH := 0.46 * h

	if down {
		if px >= cx-stemHalf && px <= cx+stemHalf && py >= b.y0 && py <= b.y1-headH+0.001 {
			return true
		}
		return inTriangle(px, py, cx-headHalf, b.y1-headH, cx+headHalf, b.y1-headH, cx, b.y1)
	}
	if px >= cx-stemHalf && px <= cx+stemHalf && py >= b.y0+headH-0.001 && py <= b.y1 {
		return true
	}
	return inTriangle(px, py, cx-headHalf, b.y0+headH, cx+headHalf, b.y0+headH, cx, b.y0)
}

func inTriangle(px, py, ax, ay, bx, by, cx, cy float64) bool {
	d1 := (px-bx)*(ay-by) - (ax-bx)*(py-by)
	d2 := (px-cx)*(by-cy) - (bx-cx)*(py-cy)
	d3 := (px-ax)*(cy-ay) - (cx-ax)*(py-ay)
	neg := d1 < 0 || d2 < 0 || d3 < 0
	pos := d1 > 0 || d2 > 0 || d3 > 0
	return !(neg && pos)
}

// insideRoundedRect reports whether (px,py) is inside the rounded rectangle using a
// signed-distance test.
func insideRoundedRect(px, py, x0, y0, x1, y1, r float64) bool {
	cx, cy := (x0+x1)/2, (y0+y1)/2
	hw, hh := (x1-x0)/2, (y1-y0)/2
	qx := math.Abs(px-cx) - (hw - r)
	qy := math.Abs(py-cy) - (hh - r)
	d := math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - r
	return d <= 0
}

// pcol is an alpha-premultiplied colour with channels in [0,1], the form used while
// compositing so anti-aliased edges blend without dark halos.
type pcol struct{ r, g, b, a float64 }

func premul(c color.RGBA) pcol {
	a := float64(c.A) / 255
	return pcol{float64(c.R) / 255 * a, float64(c.G) / 255 * a, float64(c.B) / 255 * a, a}
}

// over composites src over dst (both premultiplied).
func over(dst, src pcol) pcol {
	k := 1 - src.a
	return pcol{src.r + dst.r*k, src.g + dst.g*k, src.b + dst.b*k, src.a + dst.a*k}
}

// render samples scene over the unit square at size×size, supersampling each pixel
// to anti-alias. The result is a standard alpha-premultiplied *image.RGBA.
func render(size int, scene func(u, v float64) pcol) *image.RGBA {
	const ss = 4 // samples per axis → ss² per pixel
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var acc pcol
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					u := (float64(px) + (float64(sx)+0.5)/ss) / float64(size)
					v := (float64(py) + (float64(sy)+0.5)/ss) / float64(size)
					p := scene(u, v)
					acc.r += p.r
					acc.g += p.g
					acc.b += p.b
					acc.a += p.a
				}
			}
			n := float64(ss * ss)
			img.SetRGBA(px, py, color.RGBA{
				uint8(math.Round(acc.r / n * 255)),
				uint8(math.Round(acc.g / n * 255)),
				uint8(math.Round(acc.b / n * 255)),
				uint8(math.Round(acc.a / n * 255)),
			})
		}
	}
	return img
}
