// Package imaging prepares local files for printing: rotate, scale to a
// printer's pixel width, and convert colour to grayscale or to 1-bit black &
// white (fixed threshold or Floyd-Steinberg dithering). It also emits the
// ESC/POS raster a receipt printer understands.
//
// It has no OS dependencies on purpose: the preview the UI shows is produced by
// exactly the same pipeline that produces the bytes sent to the printer, so what
// the operator sees is what comes out of the printer.
package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // decode support
	_ "image/jpeg"
	"image/png"
	"io"
	"math"
)

// Colour modes.
const (
	ModeColor = "color" // leave the pixels alone
	ModeGray  = "gray"  // luminance only
	ModeMono  = "mono"  // 1-bit black & white
)

// Options describe how a source image is turned into something printable.
type Options struct {
	Mode      string // ModeColor | ModeGray | ModeMono
	Dither    bool   // Floyd-Steinberg instead of a hard threshold (mono only)
	Threshold int    // 1..254 cut-off used when Dither is false
	Rotate    int    // 0, 90, 180 or 270 degrees clockwise
	Invert    bool   // swap black and white (white-on-black receipts, film)
	WidthPx   int    // scale to this pixel width, preserving aspect; 0 keeps size
	MaxHeight int    // cap the scaled height (0 = no cap), preserving aspect
}

// DefaultThreshold is the mid-grey cut-off used when none is given.
const DefaultThreshold = 128

// Printable width of the common receipt heads, in dots at 203 dpi.
const (
	Width58mm = 384
	Width80mm = 576
)

// Decode reads an image in any format the agent supports (PNG, JPEG, GIF).
func Decode(r io.Reader) (image.Image, string, error) {
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, "", fmt.Errorf("this file is not an image the agent can read (PNG, JPEG and GIF are supported): %w", err)
	}
	return img, format, nil
}

// Prepare applies the whole pipeline and returns the image that will be printed.
// In ModeMono every pixel is exactly black or white, so the PNG preview shows the
// individual dots the printer will fire.
func Prepare(src image.Image, o Options) image.Image {
	img := rotate(src, o.Rotate)
	img = fit(img, o.WidthPx, o.MaxHeight)
	switch o.Mode {
	case ModeMono:
		return mono(img, o)
	case ModeGray:
		return grayscale(img, o.Invert)
	default:
		if o.Invert {
			return invertRGBA(img)
		}
		return img
	}
}

// EncodePNG renders an image for the preview pane (and for driver-backed queues,
// which print the PNG directly).
func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// --- geometry ---------------------------------------------------------------

// rotate turns the image clockwise by 0/90/180/270 degrees; other angles are
// ignored rather than approximated.
func rotate(src image.Image, deg int) image.Image {
	deg = ((deg % 360) + 360) % 360
	if deg == 0 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	var dst *image.RGBA
	switch deg {
	case 90:
		dst = image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(h-1-y, x, src.At(b.Min.X+x, b.Min.Y+y))
			}
		}
	case 180:
		dst = image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(w-1-x, h-1-y, src.At(b.Min.X+x, b.Min.Y+y))
			}
		}
	case 270:
		dst = image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(y, w-1-x, src.At(b.Min.X+x, b.Min.Y+y))
			}
		}
	default:
		return src
	}
	return dst
}

// fit scales to width px (aspect preserved), then shrinks further if the result
// would exceed maxHeight. Either bound may be 0 to skip it.
func fit(src image.Image, width, maxHeight int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return src
	}
	dw, dh := w, h
	if width > 0 && width != w {
		dw = width
		dh = int(math.Round(float64(h) * float64(width) / float64(w)))
	}
	if maxHeight > 0 && dh > maxHeight {
		dw = int(math.Round(float64(dw) * float64(maxHeight) / float64(dh)))
		dh = maxHeight
	}
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	if dw == w && dh == h {
		return src
	}
	if dw < w {
		return boxScale(src, dw, dh) // averaging keeps detail when shrinking
	}
	return bilinearScale(src, dw, dh)
}

// boxScale averages each destination pixel over the source rectangle it covers.
// Photos going down to a 576-dot receipt head lose far less than nearest-neighbour.
func boxScale(src image.Image, dw, dh int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	xRatio := float64(sw) / float64(dw)
	yRatio := float64(sh) / float64(dh)
	for dy := 0; dy < dh; dy++ {
		y0 := int(float64(dy) * yRatio)
		y1 := int(math.Ceil(float64(dy+1) * yRatio))
		if y1 > sh {
			y1 = sh
		}
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for dx := 0; dx < dw; dx++ {
			x0 := int(float64(dx) * xRatio)
			x1 := int(math.Ceil(float64(dx+1) * xRatio))
			if x1 > sw {
				x1 = sw
			}
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, bl, a, n uint64
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					cr, cg, cb, ca := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
					r += uint64(cr >> 8)
					g += uint64(cg >> 8)
					bl += uint64(cb >> 8)
					a += uint64(ca >> 8)
					n++
				}
			}
			if n == 0 {
				n = 1
			}
			dst.SetRGBA(dx, dy, color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), uint8(a / n)})
		}
	}
	return dst
}

// bilinearScale is used when enlarging (a small logo up to the head width).
func bilinearScale(src image.Image, dw, dh int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	xRatio := float64(sw-1) / math.Max(float64(dw-1), 1)
	yRatio := float64(sh-1) / math.Max(float64(dh-1), 1)
	at := func(x, y int) (float64, float64, float64, float64) {
		if x >= sw {
			x = sw - 1
		}
		if y >= sh {
			y = sh - 1
		}
		r, g, bl, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
		return float64(r >> 8), float64(g >> 8), float64(bl >> 8), float64(a >> 8)
	}
	for dy := 0; dy < dh; dy++ {
		fy := float64(dy) * yRatio
		y0 := int(fy)
		ty := fy - float64(y0)
		for dx := 0; dx < dw; dx++ {
			fx := float64(dx) * xRatio
			x0 := int(fx)
			tx := fx - float64(x0)
			r00, g00, b00, a00 := at(x0, y0)
			r10, g10, b10, a10 := at(x0+1, y0)
			r01, g01, b01, a01 := at(x0, y0+1)
			r11, g11, b11, a11 := at(x0+1, y0+1)
			mix := func(v00, v10, v01, v11 float64) uint8 {
				top := v00 + (v10-v00)*tx
				bot := v01 + (v11-v01)*tx
				return uint8(math.Round(top + (bot-top)*ty))
			}
			dst.SetRGBA(dx, dy, color.RGBA{
				mix(r00, r10, r01, r11), mix(g00, g10, g01, g11),
				mix(b00, b10, b01, b11), mix(a00, a10, a01, a11),
			})
		}
	}
	return dst
}

// --- colour -----------------------------------------------------------------

// lum returns a pixel's luminance, compositing transparency onto white (paper).
func lum(c color.Color) float64 {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return 255
	}
	// Un-premultiply, then blend onto white by alpha.
	fr := float64(r) / float64(a) * 255
	fg := float64(g) / float64(a) * 255
	fb := float64(b) / float64(a) * 255
	y := 0.299*fr + 0.587*fg + 0.114*fb
	al := float64(a) / 65535
	return y*al + 255*(1-al)
}

func grayscale(src image.Image, invert bool) *image.Gray {
	b := src.Bounds()
	dst := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			v := clamp8(lum(src.At(b.Min.X+x, b.Min.Y+y)))
			if invert {
				v = 255 - v
			}
			dst.SetGray(x, y, color.Gray{Y: v})
		}
	}
	return dst
}

func invertRGBA(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			dst.SetRGBA(x, y, color.RGBA{
				255 - uint8(r>>8), 255 - uint8(g>>8), 255 - uint8(bl>>8), uint8(a >> 8),
			})
		}
	}
	return dst
}

// mono reduces to pure black and white. Dithering trades a checkerboard of dots
// for apparent grey, which is the only way a receipt head reproduces a photo;
// a plain threshold is better for logos, text and line art.
func mono(src image.Image, o Options) *image.Gray {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewGray(image.Rect(0, 0, w, h))
	th := float64(o.Threshold)
	if o.Threshold <= 0 || o.Threshold >= 255 {
		th = DefaultThreshold
	}

	// One float row buffer per line pair keeps the diffused error exact without
	// copying the whole image into floats.
	cur := make([]float64, w)
	next := make([]float64, w)
	for x := 0; x < w; x++ {
		cur[x] = lum(src.At(b.Min.X+x, b.Min.Y))
	}
	for y := 0; y < h; y++ {
		if y+1 < h {
			for x := 0; x < w; x++ {
				next[x] = lum(src.At(b.Min.X+x, b.Min.Y+y+1))
			}
		}
		for x := 0; x < w; x++ {
			old := cur[x]
			var val float64
			if old >= th {
				val = 255
			}
			if o.Dither {
				// Push the rounding error into the neighbours that come next.
				err := old - val
				if x+1 < w {
					cur[x+1] += err * 7 / 16
				}
				if y+1 < h {
					if x > 0 {
						next[x-1] += err * 3 / 16
					}
					next[x] += err * 5 / 16
					if x+1 < w {
						next[x+1] += err * 1 / 16
					}
				}
			}
			v := uint8(0)
			if val >= 128 {
				v = 255
			}
			if o.Invert {
				v = 255 - v
			}
			dst.SetGray(x, y, color.Gray{Y: v})
		}
		cur, next = next, cur
	}
	return dst
}

func clamp8(v float64) uint8 {
	switch {
	case v <= 0:
		return 0
	case v >= 255:
		return 255
	default:
		return uint8(math.Round(v))
	}
}
