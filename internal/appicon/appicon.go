// Package appicon draws the Pinalert app mark.
//
// The mark is defined once here, as numbers on a 512 by 512 grid, and drawn by
// code: SVG writes the markup and PNG rasterises with the standard library
// only (owner rule: no image file is saved in the repo, so the icons exist as
// responses and nowhere else). The tile and mark colours are the fixed notice
// colours (#1A1D21 and #FFFFFF) because an icon cannot follow the device
// theme. The design (a white map pin with an exclamation cut out of it, on a
// dark tile) is the owner's to judge at the viewing session.
//
// Three variants exist, because the platforms ask for three kinds of icon:
//
//   - VariantAny: a rounded tile with transparent corners (favicon, manifest
//     any icons, the notification icon).
//   - VariantFullBleed: a square tile with no transparent pixel, for the
//     maskable icon and the iPhone Home Screen icon, which the system masks
//     and rounds itself.
//   - VariantBadge: the pin alone, opaque white with the exclamation as
//     transparent holes on a transparent background, cropped to the pin, for
//     the Android status bar badge.
package appicon

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// The 512 grid. Every number of the mark lives in this block and nowhere else.
const (
	// Grid is the side of the design grid.
	Grid = 512
	// TileRadius is the corner radius of the rounded tile (VariantAny).
	TileRadius = 96

	// The pin head is a circle.
	HeadCX = 256
	HeadCY = 216
	HeadR  = 120

	// The pin point is a triangle from the tip to the two tangent points of the
	// head circle, so head and point join as one drop with no kink.
	TipX      = 256
	TipY      = 412
	TangentLX = 161
	TangentRX = 351
	TangentY  = 290

	// The exclamation bar is a rounded rectangle cut out of the pin.
	BarX = 236
	BarY = 152
	BarW = 40
	BarH = 78
	BarR = 20

	// The exclamation dot is a circle cut out of the pin.
	DotCX = 256
	DotCY = 274
	DotR  = 22

	// The badge crops the pin: the BadgeCropSize unit square whose corner is at
	// BadgeCropX, BadgeCropY in the 512 grid.
	BadgeCropX    = 74
	BadgeCropY    = 72
	BadgeCropSize = 364

	// SafeRadiusFraction is the radius, as a fraction of the icon size, of the
	// circle about the centre that the mark of a maskable icon must stay inside.
	SafeRadiusFraction = 0.4
)

// The two fixed colours as hex text (for the SVG) and as pixels (for the PNG).
const (
	tileHex = "#1A1D21"
	markHex = "#FFFFFF"
)

var (
	// TileColour is the dark tile (#1A1D21, the notice background).
	TileColour = color.NRGBA{R: 0x1A, G: 0x1D, B: 0x21, A: 0xFF}
	// MarkColour is the white pin (#FFFFFF, the notice text).
	MarkColour = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
)

// Variant says which kind of icon PNG draws.
type Variant int

const (
	// VariantAny is the rounded tile with transparent corners.
	VariantAny Variant = iota
	// VariantFullBleed is the square tile with no transparent pixel.
	VariantFullBleed
	// VariantBadge is the white pin alone, alpha only, cropped to the pin.
	VariantBadge
)

const (
	minSize = 16
	maxSize = 1024
	// samples is the supersampling factor per axis: 4 by 4 samples per pixel.
	samples = 4
)

// SVG returns the markup of the any variant: the rounded tile, the white pin
// and the exclamation drawn in the tile colour. It holds no external
// reference, script or text.
func SVG() string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%[1]d" height="%[1]d" viewBox="0 0 %[1]d %[1]d">`+
		`<rect width="%[1]d" height="%[1]d" rx="%[2]d" fill="%[3]s"/>`+
		`<circle cx="%[4]d" cy="%[5]d" r="%[6]d" fill="%[7]s"/>`+
		`<polygon points="%[8]d,%[9]d %[10]d,%[11]d %[12]d,%[11]d" fill="%[7]s"/>`+
		`<rect x="%[13]d" y="%[14]d" width="%[15]d" height="%[16]d" rx="%[17]d" fill="%[3]s"/>`+
		`<circle cx="%[18]d" cy="%[19]d" r="%[20]d" fill="%[3]s"/>`+
		`</svg>`,
		Grid, TileRadius, tileHex,
		HeadCX, HeadCY, HeadR, markHex,
		TipX, TipY, TangentLX, TangentY, TangentRX,
		BarX, BarY, BarW, BarH, BarR,
		DotCX, DotCY, DotR)
}

// PNG renders the mark at size by size pixels as a PNG. The size must be from
// 16 to 1024. The output is deterministic: the same arguments give the same
// bytes.
func PNG(size int, v Variant) ([]byte, error) {
	if size < minSize || size > maxSize {
		return nil, fmt.Errorf("appicon: size %d is outside %d to %d", size, minSize, maxSize)
	}
	if v != VariantAny && v != VariantFullBleed && v != VariantBadge {
		return nil, fmt.Errorf("appicon: unknown variant %d", v)
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	// The badge crops the pin; its corner offsets differ per axis (74 and 72).
	offX, offY, span := 0.0, 0.0, float64(Grid)
	if v == VariantBadge {
		offX, offY, span = BadgeCropX, BadgeCropY, BadgeCropSize
	}
	step := span / float64(size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var mark, tile int
			for sy := 0; sy < samples; sy++ {
				gy := offY + (float64(y)+(float64(sy)+0.5)/samples)*step
				for sx := 0; sx < samples; sx++ {
					gx := offX + (float64(x)+(float64(sx)+0.5)/samples)*step
					switch sample(gx, gy, v) {
					case coverMark:
						mark++
					case coverTile:
						tile++
					}
				}
			}
			img.SetNRGBA(x, y, blend(mark, tile, v))
		}
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("appicon: encode png: %w", err)
	}
	return buf.Bytes(), nil
}

type cover int

const (
	coverNone cover = iota
	coverTile
	coverMark
)

// sample says what one grid point shows for a variant.
func sample(gx, gy float64, v Variant) cover {
	inMark := inPin(gx, gy) && !inBar(gx, gy) && !inDot(gx, gy)
	switch v {
	case VariantBadge:
		if inMark {
			return coverMark
		}
		return coverNone
	case VariantFullBleed:
		if inMark {
			return coverMark
		}
		return coverTile
	default:
		if !inRoundedRect(gx, gy, 0, 0, Grid, Grid, TileRadius) {
			return coverNone
		}
		if inMark {
			return coverMark
		}
		return coverTile
	}
}

// blend averages the samples of one pixel. mark and tile count the samples that
// showed the mark and the tile (the rest were transparent). The colour is the
// mix of the two covered colours and the alpha is the covered share. The full
// bleed variant never has an uncovered sample, so its alpha is always 255.
func blend(mark, tile int, v Variant) color.NRGBA {
	total := samples * samples
	covered := mark + tile
	if covered == 0 {
		return color.NRGBA{}
	}
	alpha := uint8((covered*255 + total/2) / total)
	if v == VariantBadge {
		return color.NRGBA{R: 255, G: 255, B: 255, A: alpha}
	}
	mix := func(m, t uint8) uint8 {
		return uint8((int(m)*mark + int(t)*tile + covered/2) / covered)
	}
	return color.NRGBA{
		R: mix(MarkColour.R, TileColour.R),
		G: mix(MarkColour.G, TileColour.G),
		B: mix(MarkColour.B, TileColour.B),
		A: alpha,
	}
}

// inPin reports whether the point is in the pin head or the pin point.
func inPin(x, y float64) bool {
	return inCircle(x, y, HeadCX, HeadCY, HeadR) ||
		inTriangle(x, y, TipX, TipY, TangentLX, TangentY, TangentRX, TangentY)
}

func inBar(x, y float64) bool {
	return inRoundedRect(x, y, BarX, BarY, BarW, BarH, BarR)
}

func inDot(x, y float64) bool {
	return inCircle(x, y, DotCX, DotCY, DotR)
}

func inCircle(x, y, cx, cy, r float64) bool {
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

// inTriangle uses the signs of the three edge cross products: the point is
// inside when they do not disagree.
func inTriangle(x, y, ax, ay, bx, by, cx, cy float64) bool {
	d1 := (x-bx)*(ay-by) - (ax-bx)*(y-by)
	d2 := (x-cx)*(by-cy) - (bx-cx)*(y-cy)
	d3 := (x-ax)*(cy-ay) - (cx-ax)*(y-ay)
	neg := d1 < 0 || d2 < 0 || d3 < 0
	pos := d1 > 0 || d2 > 0 || d3 > 0
	return !(neg && pos)
}

// inRoundedRect reports whether the point is in the rectangle with corner
// radius r.
func inRoundedRect(x, y, rx, ry, w, h, r float64) bool {
	if x < rx || x > rx+w || y < ry || y > ry+h {
		return false
	}
	// Distance into the nearest corner square, if the point is in one.
	var cx, cy float64
	switch {
	case x < rx+r:
		cx = rx + r
	case x > rx+w-r:
		cx = rx + w - r
	default:
		return true
	}
	switch {
	case y < ry+r:
		cy = ry + r
	case y > ry+h-r:
		cy = ry + h - r
	default:
		return true
	}
	return inCircle(x, y, cx, cy, r)
}

// Set holds every PNG the site serves, rendered once, with an ETag for each.
type Set struct {
	Any192      []byte
	Any512      []byte
	Maskable512 []byte
	Apple180    []byte
	Badge96     []byte

	Any192ETag      string
	Any512ETag      string
	Maskable512ETag string
	Apple180ETag    string
	Badge96ETag     string
}

// Render draws the five PNGs of the set.
func Render() (*Set, error) {
	s := &Set{}
	jobs := []struct {
		bytes *[]byte
		etag  *string
		size  int
		v     Variant
	}{
		{&s.Any192, &s.Any192ETag, 192, VariantAny},
		{&s.Any512, &s.Any512ETag, 512, VariantAny},
		{&s.Maskable512, &s.Maskable512ETag, 512, VariantFullBleed},
		{&s.Apple180, &s.Apple180ETag, 180, VariantFullBleed},
		{&s.Badge96, &s.Badge96ETag, 96, VariantBadge},
	}
	for _, j := range jobs {
		b, err := PNG(j.size, j.v)
		if err != nil {
			return nil, err
		}
		*j.bytes = b
		*j.etag = ETag(b)
	}
	return s, nil
}

// ETag is a quoted strong ETag: a prefix of the hex sha256 of the bytes.
func ETag(b []byte) string {
	sum := sha256.Sum256(b)
	return `"` + hex.EncodeToString(sum[:8]) + `"`
}
