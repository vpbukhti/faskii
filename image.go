package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"golang.org/x/image/draw"
)

func readImage(imageFilepath string) (image.Image, error) {
	inputFile, err := os.Open(imageFilepath)
	if err != nil {
		return nil, fmt.Errorf("unable to open file: %w", err)
	}
	defer inputFile.Close()

	img, _, err := image.Decode(inputFile)
	if err != nil {
		return nil, fmt.Errorf("unable to decode image: %w", err)
	}

	return img, nil
}

func writeImage(imageFilepath string, img image.Image) error {
	resFile, err := os.Create(imageFilepath)
	if err != nil {
		return fmt.Errorf("unable to open file: %w", err)
	}
	defer resFile.Close()

	err = png.Encode(resFile, img)
	if err != nil {
		return fmt.Errorf("unbale to encode file: %w", err)
	}

	return nil
}

func fit(srcWidth, srcHeight, dstWidth, dstHeight int) (int, int) {
	if float64(srcWidth)/float64(srcHeight) > float64(dstWidth)/float64(dstHeight) {
		return dstWidth, int(float64(srcHeight) * (float64(dstWidth) / float64(srcWidth)))
	} else {
		return int(float64(srcWidth) * (float64(dstHeight) / float64(srcHeight))), dstHeight
	}
}

func scale(src image.Image, scaledWidth, scaledHeight int) draw.Image {
	res := image.NewRGBA(image.Rect(0, 0, scaledWidth, scaledHeight))

	draw.NearestNeighbor.Scale(res, res.Rect, src, src.Bounds(), draw.Over, &draw.Options{})

	return res
}

func normalize(src draw.Image) draw.Image {
	avg := float64(avgIntensity(src))

	for y := range src.Bounds().Dy() {
		for x := range src.Bounds().Dx() {
			r, g, b, a := src.At(x, y).RGBA()
			src.Set(x, y, color.RGBA{
				R: uint8(min(float64(r>>8)*(avg/128.0), 255.0)),
				G: uint8(min(float64(g>>8)*(avg/128.0), 255.0)),
				B: uint8(min(float64(b>>8)*(avg/128.0), 255.0)),
				A: uint8(a),
			})
		}
	}

	return src
}

type imagePart struct {
	rect      image.Rectangle
	img       draw.Image
	intensity uint32
}

func split(src image.Image, partWidth, partHeight int) [][]imagePart {
	srcWidth := src.Bounds().Max.X
	srcHeight := src.Bounds().Max.Y

	if srcWidth%partWidth != 0 || srcHeight%partHeight != 0 {
		panic(fmt.Errorf("unable to split: dimetions must be proportional: (%d, %d), (%d, %d)", srcWidth, srcHeight, partWidth, partHeight))
	}

	res := [][]imagePart{}
	var avg uint64

	for j := range srcHeight / partHeight {
		row := []imagePart{}
		for i := range srcWidth / partWidth {
			part := imagePart{
				rect: image.Rect(i*partWidth, j*partHeight, i*partWidth+partWidth, j*partHeight+partHeight),
				img:  image.NewRGBA(image.Rect(0, 0, partWidth, partHeight)),
			}

			draw.Copy(part.img, image.Pt(0, 0), src, part.rect, draw.Over, &draw.Options{})

			in := avgIntensity(part.img)
			part.intensity = in
			avg += uint64(in)

			row = append(row, part)
		}
		res = append(res, row)
	}

	return res
}

func greyColor(c color.Color) uint32 {
	r, g, b, _ := c.RGBA()

	r &= r >> 8
	g &= g >> 8
	b &= b >> 8

	return (r + g + b) / 3
}

func avgIntensity(img image.Image) uint32 {
	var avg uint64

	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			gc := greyColor(img.At(x, y))
			avg += uint64(gc)
		}
	}

	return uint32(avg / (uint64(img.Bounds().Dx()) * uint64(img.Bounds().Dy())))
}

func join(parts []imagePart) image.Image {
	var width, height int
	for _, part := range parts {
		if part.rect.Max.X > width {
			width = part.rect.Max.X
		}
		if part.rect.Max.Y > height {
			height = part.rect.Max.Y
		}
	}

	res := image.NewRGBA(image.Rect(0, 0, width, height))

	for _, part := range parts {
		draw.Copy(res, part.rect.Min, part.img, part.img.Bounds(), draw.Over, &draw.Options{})
	}

	return res
}
