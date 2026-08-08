package imaging

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

// solid builds a w×h image filled with one colour.
func solid(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestFitScalesToPrinterWidthKeepingAspect(t *testing.T) {
	src := solid(3024, 4032, color.White) // a phone photo
	got := Prepare(src, Options{Mode: ModeGray, WidthPx: Width80mm})
	b := got.Bounds()
	if b.Dx() != 576 {
		t.Errorf("width = %d, want the 80mm head width of 576 dots", b.Dx())
	}
	if want := 768; b.Dy() != want { // 4032 * 576 / 3024
		t.Errorf("height = %d, want %d (aspect must be preserved)", b.Dy(), want)
	}
}

func TestFitCapsHeight(t *testing.T) {
	src := solid(100, 10000, color.White) // a very tall receipt image
	got := Prepare(src, Options{Mode: ModeGray, WidthPx: 576, MaxHeight: 4000})
	b := got.Bounds()
	if b.Dy() > 4000 {
		t.Errorf("height = %d, want it capped at 4000", b.Dy())
	}
	if b.Dx() >= 576 {
		t.Errorf("width = %d, want it shrunk with the height to keep aspect", b.Dx())
	}
}

func TestFitLeavesSizeAloneWhenWidthIsZero(t *testing.T) {
	src := solid(200, 100, color.White)
	b := Prepare(src, Options{Mode: ModeColor}).Bounds()
	if b.Dx() != 200 || b.Dy() != 100 {
		t.Errorf("size = %dx%d, want the original 200x100", b.Dx(), b.Dy())
	}
}

func TestMonoThreshold(t *testing.T) {
	// Mid-grey: below a high threshold it is black, above a low one it is white.
	grey := solid(4, 4, color.RGBA{128, 128, 128, 255})

	dark := Prepare(grey, Options{Mode: ModeMono, Threshold: 200})
	if got := dark.At(0, 0); !isBlack(got) {
		t.Errorf("with threshold 200, mid-grey must print black, got %v", got)
	}
	light := Prepare(grey, Options{Mode: ModeMono, Threshold: 60})
	if got := light.At(0, 0); isBlack(got) {
		t.Errorf("with threshold 60, mid-grey must stay white, got %v", got)
	}
}

func TestMonoIsPurelyBlackAndWhite(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			v := uint8(x * 8) // a gradient across the image
			src.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	out := Prepare(src, Options{Mode: ModeMono, Dither: true})
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			r, _, _, _ := out.At(x, y).RGBA()
			if v := uint8(r >> 8); v != 0 && v != 255 {
				t.Fatalf("pixel (%d,%d) = %d: a mono image must contain only 0 and 255", x, y, v)
			}
		}
	}
}

func TestDitherRendersMidGreyAsMixedDots(t *testing.T) {
	grey := solid(64, 64, color.RGBA{128, 128, 128, 255})
	out := Prepare(grey, Options{Mode: ModeMono, Dither: true})
	var black, white int
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if isBlack(out.At(x, y)) {
				black++
			} else {
				white++
			}
		}
	}
	if black == 0 || white == 0 {
		t.Fatalf("dithered mid-grey = %d black / %d white, want a mix of both", black, white)
	}
	// Roughly half the dots should fire for 50% grey.
	ratio := float64(black) / float64(black+white)
	if ratio < 0.35 || ratio > 0.65 {
		t.Errorf("black coverage = %.2f, want near 0.5 for mid-grey", ratio)
	}
}

func TestThresholdWithoutDitherIsFlat(t *testing.T) {
	grey := solid(16, 16, color.RGBA{128, 128, 128, 255})
	out := Prepare(grey, Options{Mode: ModeMono, Threshold: 128})
	first := isBlack(out.At(0, 0))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if isBlack(out.At(x, y)) != first {
				t.Fatal("without dithering a flat grey must map to one solid tone")
			}
		}
	}
}

func TestRotate(t *testing.T) {
	src := solid(10, 4, color.White)
	if b := Prepare(src, Options{Mode: ModeColor, Rotate: 90}).Bounds(); b.Dx() != 4 || b.Dy() != 10 {
		t.Errorf("90° = %dx%d, want 4x10", b.Dx(), b.Dy())
	}
	if b := Prepare(src, Options{Mode: ModeColor, Rotate: 180}).Bounds(); b.Dx() != 10 || b.Dy() != 4 {
		t.Errorf("180° = %dx%d, want 10x4", b.Dx(), b.Dy())
	}
}

func TestInvertSwapsBlackAndWhite(t *testing.T) {
	out := Prepare(solid(2, 2, color.White), Options{Mode: ModeMono, Invert: true})
	if !isBlack(out.At(0, 0)) {
		t.Error("inverted white must print black")
	}
}

func TestTransparentPixelsPrintAsPaper(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2)) // zero value: fully transparent
	out := Prepare(src, Options{Mode: ModeMono})
	if isBlack(out.At(0, 0)) {
		t.Error("transparent areas must come out white, not as a solid black block")
	}
}

func TestESCPOSHeaderAndBitPacking(t *testing.T) {
	// One row, 8 pixels: only the leftmost is black.
	img := image.NewGray(image.Rect(0, 0, 8, 1))
	for x := 0; x < 8; x++ {
		img.SetGray(x, 0, color.Gray{Y: 255})
	}
	img.SetGray(0, 0, color.Gray{Y: 0})

	got := ESCPOS(img, ESCPOSOptions{Align: AlignCenter, FeedDots: 120, Cut: true})

	want := []byte{
		0x1b, 0x40, // ESC @
		0x1b, 0x61, 0x01, // ESC a 1 (centre)
		0x1d, 0x76, 0x30, 0x00, // GS v 0 m=0
		0x01, 0x00, // one byte per row
		0x01, 0x00, // one row
		0x80,             // leftmost dot fires
		0x1b, 0x4a, 0x78, // ESC J 120 (15 mm of tear-off feed)
		0x1d, 0x56, 0x42, 0x00, // GS V 66 0 (partial cut)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ESC/POS output\n got %#v\nwant %#v", got, want)
	}
}

func TestTearOffFeedIsSplitAcrossCommands(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 8, 1))
	// 60 mm is 480 dots, past the 255 a single ESC J can carry.
	got := ESCPOS(img, ESCPOSOptions{FeedDots: MMToDots(60)})
	if n := bytes.Count(got, escFeedN); n != 2 {
		t.Errorf("feed commands = %d, want 2 for a 480-dot feed", n)
	}
	var total int
	for i := 0; i+2 < len(got); i++ {
		if got[i] == 0x1b && got[i+1] == 0x4a {
			total += int(got[i+2])
		}
	}
	if want := MMToDots(60); total != want {
		t.Errorf("total feed = %d dots, want %d", total, want)
	}
}

func TestNoFeedCommandWhenTearOffIsZero(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 8, 1))
	if got := ESCPOS(img, ESCPOSOptions{FeedDots: 0}); bytes.Contains(got, escFeedN) {
		t.Error("a zero tear-off must not emit a feed command")
	}
}

func TestMillimetreConversion(t *testing.T) {
	// 203 dpi is very close to 8 dots per millimetre.
	cases := []struct{ mm, dots int }{{0, 0}, {1, 8}, {15, 120}, {60, 480}}
	for _, c := range cases {
		if got := MMToDots(c.mm); got != c.dots {
			t.Errorf("MMToDots(%d) = %d, want %d", c.mm, got, c.dots)
		}
		if got := DotsToMM(c.dots); got != c.mm {
			t.Errorf("DotsToMM(%d) = %d, want %d", c.dots, got, c.mm)
		}
	}
	if got := MMToDots(-5); got != 0 {
		t.Errorf("MMToDots(-5) = %d, want 0", got)
	}
}

func TestESCPOSSplitsTallImagesIntoBands(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 8, 300)) // taller than one band
	got := ESCPOS(img, ESCPOSOptions{})
	if n := bytes.Count(got, []byte{0x1d, 0x76, 0x30, 0x00}); n != 3 {
		t.Errorf("raster commands = %d, want 3 bands for 300 rows at %d rows each", n, bandRows)
	}
}

func TestDecodeRejectsNonImages(t *testing.T) {
	if _, _, err := Decode(bytes.NewReader([]byte("%PDF-1.7\n"))); err == nil {
		t.Fatal("a PDF must not decode as an image")
	}
}

func isBlack(c color.Color) bool {
	r, _, _, _ := c.RGBA()
	return r>>8 < 128
}
