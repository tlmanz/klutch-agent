package imaging

import (
	"bytes"
	"image"
)

// ESC/POS control sequences. A receipt printer attached to a raw queue takes
// these verbatim; a driver-backed queue never sees them.
var (
	escInit   = []byte{0x1b, 0x40}       // ESC @      reset
	escAlign  = []byte{0x1b, 0x61}       // ESC a n    0 left, 1 centre, 2 right
	escFeedN  = []byte{0x1b, 0x4a}       // ESC J n    feed n motion units (n/203")
	gsCut     = []byte{0x1d, 0x56, 0x42} // GS V 66 n  feed to cut position and partial cut
	gsRasterV = []byte{0x1d, 0x76, 0x30} // GS v 0 m   raster bit image
)

// DotsPerMM is the vertical resolution of a standard 203 dpi receipt head:
// 203 / 25.4 ≈ 8 dots per millimetre. Feed distances are expressed in
// millimetres everywhere above this package and converted here.
const DotsPerMM = 203.0 / 25.4

// MMToDots converts a millimetre distance to printer dots.
func MMToDots(mm int) int {
	if mm <= 0 {
		return 0
	}
	return int(float64(mm)*DotsPerMM + 0.5)
}

// DotsToMM converts printer dots back to whole millimetres.
func DotsToMM(dots int) int {
	if dots <= 0 {
		return 0
	}
	return int(float64(dots)/DotsPerMM + 0.5)
}

// maxFeedUnits is the largest single ESC J feed (the argument is one byte), so
// longer feeds are emitted as repeated commands.
const maxFeedUnits = 255

// Alignment values for ESCPOSOptions.Align.
const (
	AlignLeft   = 0
	AlignCenter = 1
	AlignRight  = 2
)

// ESCPOSOptions control the receipt-printer envelope around the raster.
type ESCPOSOptions struct {
	Align int // AlignLeft | AlignCenter | AlignRight

	// FeedDots is blank paper fed after the image. It exists because the tear bar
	// sits a fixed distance past the print head: without it the last lines are
	// still inside the mechanism when you tear, and you tear through them. No
	// ESC/POS command reports that distance, so it is a per-printer measurement.
	FeedDots int

	// Cut requests a partial cut. Printers with a cutter feed to their own cut
	// position first (that offset is built into the firmware); printers without
	// one ignore the command, which is why FeedDots is what makes hand-tearing
	// safe.
	Cut bool
}

// bandRows is how many raster rows go in one GS v 0 command. Receipt printers
// have small input buffers and some stall on a full-page raster, so the image is
// streamed in bands; this is the value every reference implementation uses.
const bandRows = 128

// ESCPOS renders an image as ESC/POS raster data. Any pixel darker than mid-grey
// fires a dot, so callers normally pass an image already reduced by Prepare with
// ModeMono; passing a greyscale or colour image still works and thresholds here.
func ESCPOS(img image.Image, o ESCPOSOptions) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	var buf bytes.Buffer
	buf.Write(escInit)
	if o.Align >= AlignLeft && o.Align <= AlignRight {
		buf.Write(escAlign)
		buf.WriteByte(byte(o.Align))
	}
	if w == 0 || h == 0 {
		return buf.Bytes()
	}

	widthBytes := (w + 7) / 8
	for y0 := 0; y0 < h; y0 += bandRows {
		rows := bandRows
		if y0+rows > h {
			rows = h - y0
		}
		buf.Write(gsRasterV)
		buf.WriteByte(0) // m = 0: normal density
		buf.WriteByte(byte(widthBytes & 0xff))
		buf.WriteByte(byte(widthBytes >> 8))
		buf.WriteByte(byte(rows & 0xff))
		buf.WriteByte(byte(rows >> 8))
		for y := y0; y < y0+rows; y++ {
			for xb := 0; xb < widthBytes; xb++ {
				var bits byte
				for bit := 0; bit < 8; bit++ {
					x := xb*8 + bit
					if x >= w {
						break // pixels past the edge stay white
					}
					if lum(img.At(b.Min.X+x, b.Min.Y+y)) < 128 {
						bits |= 1 << (7 - bit)
					}
				}
				buf.WriteByte(bits)
			}
		}
	}

	buf.Write(Finish(o.FeedDots, o.Cut))
	return buf.Bytes()
}

// Finish is how a receipt ends: the tear-off feed, then an optional cut.
//
// Separate from ESCPOS because a raster drawn ELSEWHERE has to end the same way.
// Klutch's receipts are rendered by the backend - it owns the layout, the fonts
// and the languages - and arrive here as finished ESC/POS bytes, but how the paper
// leaves the machine is not part of the document. It is a fact about this chassis,
// no ESC/POS command reports it, and only the agent is standing next to the
// printer, so the ending is appended here whoever drew the pixels.
//
// ESC J feeds an exact number of dots, unlike ESC d, whose "lines" depend on the
// current line spacing and so differ between models. Its argument is a single
// byte, hence the loop: a longer feed is several commands, never a truncated one.
func Finish(feedDots int, cut bool) []byte {
	var buf bytes.Buffer
	for left := feedDots; left > 0; {
		n := left
		if n > maxFeedUnits {
			n = maxFeedUnits
		}
		buf.Write(escFeedN)
		buf.WriteByte(byte(n))
		left -= n
	}
	if cut {
		buf.Write(gsCut)
		buf.WriteByte(0)
	}
	return buf.Bytes()
}
