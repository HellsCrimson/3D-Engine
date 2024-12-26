package textures

import (
	"embed"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/go-gl/gl/v4.6-core/gl"
)

//go:embed *.jpg
var textureDir embed.FS

type Texture struct {
	Width  int32
	Height int32
	Data   []byte
}

func Load(name string) uint32 {
	var textureId uint32
	gl.GenTextures(1, &textureId)
	gl.BindTexture(gl.TEXTURE_2D, textureId)

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	texture := getImage(name)

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, texture.Width, texture.Height, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(texture.Data))
	gl.GenerateMipmap(gl.TEXTURE_2D)

	gl.BindTexture(gl.TEXTURE_2D, 0)

	return textureId
}

func getImage(name string) *Texture {
	file, err := textureDir.Open(name)
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
