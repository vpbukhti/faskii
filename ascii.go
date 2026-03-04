package main

import (
	"image"
	"image/color"
	"math"
	"os"
	"slices"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	charHeight = 13
	charWidth  = 6
)

type char struct {
	char          string
	img           image.Image
	intensity     uint32
	origIntensity uint32
}

var charset = generatePrintableAsciiCharset()

func stretchAndFlatten(x uint32, maxInput uint32, gamma float64) uint32 {
	if x > maxInput {
		x = maxInput
	}
	normalized := float64(x) / float64(maxInput)
	adjusted := math.Pow(normalized, gamma) * 255.0
	return uint32(adjusted + 0.5)
}

func generatePrintableAsciiCharset() []char {
	res := []char{}
	var maxIntensity uint32

	for i := 32; i <= 126; i++ {
		c := string(rune(i))

		img := image.NewRGBA(image.Rect(0, 0, charWidth, charHeight))
		draw.Draw(img, img.Bounds(), &image.Uniform{color.Black}, image.Point{}, draw.Src)

		d := &font.Drawer{
			Dst:  img,
			Src:  image.White,
			Face: basicfont.Face7x13,
			Dot: fixed.Point26_6{
				X: fixed.I(0),
				Y: fixed.I(11),
			},
		}

		d.DrawString(c)

		in := avgIntensity(img)
		if in > maxIntensity {
			maxIntensity = in
		}

		res = append(res, char{
			char:          c,
			img:           img,
			intensity:     in,
			origIntensity: in,
		})
	}

	for i := range res {
		res[i].intensity = stretchAndFlatten(res[i].intensity, maxIntensity, 1.1)
	}

	slices.SortFunc(res, func(a, b char) int {
		return int(a.intensity) - int(b.intensity)
	})

	return res
}

func processImage(src image.Image, terminalDims terminalDimentions) ([][]char, error) {
	termWidthPixels, termHeightPixels := terminalDims.width*charWidth, terminalDims.height*charHeight

	resWidth, resHeight := fit(src.Bounds().Dx(), src.Bounds().Dy(), termWidthPixels, termHeightPixels)

	// round down to allign with char dims
	resWidth = resWidth - resWidth%charWidth
	resHeight = resHeight - resHeight%charHeight

	tempSrc := scale(src, resWidth, resHeight)

	tempSrc = normalize(tempSrc)

	writeImage(getOutputFilename(os.Args[1]), tempSrc)

	parts := split(tempSrc, charWidth, charHeight)

	resParts := make([][]char, 0, len(parts))

	for _, row := range parts {
		resRow := make([]char, 0, len(row))
		for _, part := range row {
			char := asciiyze(part)
			resRow = append(resRow, char)
		}
		resParts = append(resParts, resRow)
	}

	return resParts, nil
}

func abs(v int32) uint32 {
	if v < 0 {
		return uint32(-v)
	}
	return uint32(v)
}

func asciiyze(part imagePart) char {
	var minIntensity uint32 = math.MaxUint32
	var bestChar char

	slices.BinarySearchFunc(charset, part, func(c char, part imagePart) int {
		intensity := abs(int32(part.intensity) - int32(c.intensity))

		if intensity < minIntensity {
			minIntensity = intensity
			bestChar = c
		}

		return int(c.intensity) - int(part.intensity)
	})

	return bestChar
}
