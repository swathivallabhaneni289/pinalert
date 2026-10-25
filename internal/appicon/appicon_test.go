package appicon

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"testing"
)

// decode reads one PNG and returns it with its size, failing the test when the
// bytes are not a square PNG.
func decode(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != bounds.Dy() {
		t.Fatalf("icon is %dx%d, want a square", bounds.Dx(), bounds.Dy())
	}
	return img
}

// px returns the non premultiplied colour of the pixel at x, y.
func px(img image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}

// gridPx returns the pixel under a point given in 512 grid units.
func gridPx(img image.Image, gx, gy float64) color.NRGBA {
	scale := float64(img.Bounds().Dx()) / Grid
	return px(img, int(gx*scale), int(gy*scale))
}

func mustPNG(t *testing.T, size int, v Variant) []byte {
	t.Helper()
	b, err := PNG(size, v)
	if err != nil {
		t.Fatalf("PNG(%d, %d): %v", size, v, err)
	}
	return b
}

func TestAppIconsAreDeterministic(t *testing.T) {
	a, err := Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	b, err := Render()
	if err != nil {
		t.Fatalf("Render again: %v", err)
	}
	pairs := []struct {
		name string
		x, y []byte
	}{
		{"any192", a.Any192, b.Any192},
		{"any512", a.Any512, b.Any512},
		{"maskable512", a.Maskable512, b.Maskable512},
		{"apple180", a.Apple180, b.Apple180},
		{"badge96", a.Badge96, b.Badge96},
	}
	for _, p := range pairs {
		if len(p.x) == 0 {
			t.Errorf("%s is empty", p.name)
		}
		if !bytes.Equal(p.x, p.y) {
			t.Errorf("%s differs between two renders", p.name)
		}
	}
	if a.Any192ETag == "" || a.Any192ETag != b.Any192ETag {
		t.Errorf("Any192ETag not stable: %q vs %q", a.Any192ETag, b.Any192ETag)
	}
	if a.Any192ETag == a.Any512ETag {
		t.Errorf("two different images share the ETag %q", a.Any192ETag)
	}
	if SVG() != SVG() {
		t.Error("SVG differs between two calls")
	}
}

func TestAppIconCornersAndOpacity(t *testing.T) {
	for _, size := range []int{192, 512} {
		img := decode(t, mustPNG(t, size, VariantAny))
		if img.Bounds().Dx() != size {
			t.Fatalf("any icon is %d wide, want %d", img.Bounds().Dx(), size)
		}
		if c := px(img, 0, 0); c.A != 0 {
			t.Errorf("any %d: top left alpha = %d, want 0", size, c.A)
		}
		if c := px(img, size-1, size-1); c.A != 0 {
			t.Errorf("any %d: bottom right alpha = %d, want 0", size, c.A)
		}
		// A point inside the pin head that is not under the exclamation.
		if c := gridPx(img, 176, 216); c != (color.NRGBA{255, 255, 255, 255}) {
			t.Errorf("any %d: head pixel = %v, want opaque white", size, c)
		}
		// The bar is cut out in the tile colour.
		if c := gridPx(img, 256, 190); c != TileColour {
			t.Errorf("any %d: exclamation bar pixel = %v, want the tile colour %v", size, c, TileColour)
		}
		// The tile away from the pin is opaque tile colour.
		if c := gridPx(img, 256, 40); c != TileColour {
			t.Errorf("any %d: tile pixel = %v, want %v", size, c, TileColour)
		}
	}

	for _, tc := range []struct {
		name string
		size int
	}{{"maskable", 512}, {"apple", 180}} {
		img := decode(t, mustPNG(t, tc.size, VariantFullBleed))
		if img.Bounds().Dx() != tc.size {
			t.Fatalf("%s icon is %d wide, want %d", tc.name, img.Bounds().Dx(), tc.size)
		}
		for y := 0; y < tc.size; y++ {
			for x := 0; x < tc.size; x++ {
				if c := px(img, x, y); c.A != 255 {
					t.Fatalf("%s: pixel %d,%d has alpha %d, want 255", tc.name, x, y, c.A)
				}
			}
		}
		if c := px(img, 0, 0); c != TileColour {
			t.Errorf("%s: corner = %v, want the tile colour %v", tc.name, c, TileColour)
		}
	}
}

func TestMaskableSafeZone(t *testing.T) {
	for _, size := range []int{512, 180} {
		img := decode(t, mustPNG(t, size, VariantFullBleed))
		limit := SafeRadiusFraction * float64(size)
		mid := float64(size) / 2
		worst := 0.0
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				if px(img, x, y) == TileColour {
					continue
				}
				d := math.Hypot(float64(x)+0.5-mid, float64(y)+0.5-mid)
				if d > worst {
					worst = d
				}
				if d > limit {
					t.Fatalf("size %d: mark pixel %d,%d is %.1f from the centre, limit %.1f", size, x, y, d, limit)
				}
			}
		}
		if worst == 0 {
			t.Errorf("size %d: no mark pixel found at all", size)
		}
	}
}

func TestBadgeIsAlphaOnly(t *testing.T) {
	img := decode(t, mustPNG(t, 96, VariantBadge))
	if img.Bounds().Dx() != 96 {
		t.Fatalf("badge is %d wide, want 96", img.Bounds().Dx())
	}
	opaque := 0
	for y := 0; y < 96; y++ {
		for x := 0; x < 96; x++ {
			c := px(img, x, y)
			if c.A == 0 {
				continue
			}
			if c.R != 255 || c.G != 255 || c.B != 255 {
				t.Fatalf("badge pixel %d,%d = %v, want white at any alpha", x, y, c)
			}
			if c.A == 255 {
				opaque++
			}
		}
	}
	if opaque == 0 {
		t.Error("badge has no opaque white pixel")
	}
	if c := px(img, 0, 0); c.A != 0 {
		t.Errorf("badge corner alpha = %d, want 0", c.A)
	}
	// The exclamation is a hole, so the bar and the dot are transparent.
	scale := float64(BadgeCropSize) / 96
	at := func(gx, gy float64) color.NRGBA {
		return px(img, int((gx-BadgeCropX)/scale), int((gy-BadgeCropY)/scale))
	}
	if c := at(256, 190); c.A != 0 {
		t.Errorf("badge exclamation bar alpha = %d, want 0", c.A)
	}
	if c := at(DotCX, DotCY); c.A != 0 {
		t.Errorf("badge exclamation dot alpha = %d, want 0", c.A)
	}
	if c := at(176, 216); c != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("badge head pixel = %v, want opaque white", c)
	}
}

func TestSvgAndRasterShareGeometry(t *testing.T) {
	svg := SVG()
	want := []string{
		fmt.Sprintf(`viewBox="0 0 %d %d"`, Grid, Grid),
		fmt.Sprintf(`<rect width="%d" height="%d" rx="%d" fill="#1A1D21"/>`, Grid, Grid, TileRadius),
		fmt.Sprintf(`<circle cx="%d" cy="%d" r="%d" fill="#FFFFFF"/>`, HeadCX, HeadCY, HeadR),
		fmt.Sprintf(`<polygon points="%d,%d %d,%d %d,%d" fill="#FFFFFF"/>`, TipX, TipY, TangentLX, TangentY, TangentRX, TangentY),
		fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" rx="%d" fill="#1A1D21"/>`, BarX, BarY, BarW, BarH, BarR),
		fmt.Sprintf(`<circle cx="%d" cy="%d" r="%d" fill="#1A1D21"/>`, DotCX, DotCY, DotR),
	}
	for _, w := range want {
		if !strings.Contains(svg, w) {
			t.Errorf("SVG lacks %s", w)
		}
	}
	for _, banned := range []string{"<script", "<text", "href", "url(", "<image", "<style"} {
		if strings.Contains(svg, banned) {
			t.Errorf("SVG holds %q", banned)
		}
	}
	// The raster uses the same numbers: the pin point is white just above the
	// tip and the tile just beside it, and the head edge sits at HeadCX-HeadR.
	img := decode(t, mustPNG(t, 512, VariantAny))
	if c := px(img, TipX, TipY-6); c != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("just above the tip = %v, want white", c)
	}
	if c := px(img, TipX+30, TipY-6); c != TileColour {
		t.Errorf("beside the tip = %v, want the tile colour", c)
	}
	if c := px(img, HeadCX-HeadR+4, HeadCY); c != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("inside the head edge = %v, want white", c)
	}
	if c := px(img, HeadCX-HeadR-4, HeadCY); c != TileColour {
		t.Errorf("outside the head edge = %v, want the tile colour", c)
	}
}

func TestPNGSizeBounds(t *testing.T) {
	for _, size := range []int{-1, 0, 15, 1025} {
		if _, err := PNG(size, VariantAny); err == nil {
			t.Errorf("PNG(%d) returned no error", size)
		}
	}
	for _, size := range []int{16, 1024} {
		if size == 1024 && testing.Short() {
			continue
		}
		if _, err := PNG(size, VariantAny); err != nil {
			t.Errorf("PNG(%d): %v", size, err)
		}
	}
	if _, err := PNG(96, Variant(99)); err == nil {
		t.Error("an unknown variant returned no error")
	}
}
