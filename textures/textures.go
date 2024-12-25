package textures

import (
	"embed"
	"fmt"
	"image"
	"image/jpeg"
)

//go:embed *.jpg
var textureDir embed.FS

type Texture struct {
	Width  int32
	Height int32
	Data   []byte
}

func Load(name string) *Texture {
	file, err := textureDir.Open(name + ".jpg")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil
	}
	defer file.Close()

	img, err := jpeg.Decode(file)
	if err != nil {
		fmt.Println("Error decoding JPG:", err)
		return nil
	}

	rgba := imageToRGBA(img)

	return &Texture{
		Width:  int32(rgba.Rect.Dx()),
		Height: int32(rgba.Rect.Dy()),
		Data:   rgba.Pix,
	}
}

func imageToRGBA(img image.Image) *image.RGBA {
	rgba, ok := img.(*image.RGBA)
	if ok {
		return rgba
	}

	bounds := img.Bounds()
	rgba = image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}
	return rgba
}
