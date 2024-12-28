package textures

import (
	"3d-engine/utils"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	"github.com/dblezek/tga"
	"github.com/go-gl/gl/v4.6-core/gl"
)

type Texture struct {
	Width  int32
	Height int32
	Data   []byte
}

func Load(name string) (uint32, error) {
	var textureId uint32
	gl.GenTextures(1, &textureId)
	gl.BindTexture(gl.TEXTURE_2D, textureId)

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	texture, err := getImage(name)
	if err != nil {
		return 0, err
	}

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, texture.Width, texture.Height, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(texture.Data))
	gl.GenerateMipmap(gl.TEXTURE_2D)

	gl.BindTexture(gl.TEXTURE_2D, 0)

	return textureId, nil
}

func getImage(name string) (*Texture, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, utils.Logger().Errorf("Error opening file: %s\n", err)
	}
	defer file.Close()

	extension := filepath.Ext(name)

	img, err := decodeImage(extension, file)
	if err != nil {
		return nil, err
	}

	// flipped := flipImageVertically(img)
	flipped := imageToRGBA(img)

	return &Texture{
		Width:  int32(flipped.Rect.Dx()),
		Height: int32(flipped.Rect.Dy()),
		Data:   flipped.Pix,
	}, nil
}

func decodeImage(extension string, file *os.File) (image.Image, error) {
	var img image.Image
	var err error

	if extension == ".jpg" || extension == ".jpeg" {
		img, err = jpeg.Decode(file)
		if err != nil {
			return nil, utils.Logger().Errorf("Error decoding JPG: %s\n", err)
		}
	} else if extension == ".png" {
		img, err = png.Decode(file)
		if err != nil {
			return nil, utils.Logger().Errorf("Error decoding PNG: %s\n", err)
		}
	} else if extension == ".tga" {
		img, err = tga.Decode(file)
		if err != nil {
			return nil, utils.Logger().Errorf("Error decoding TGA: %s\n", err)
		}
	} else {
		return nil, utils.Logger().Errorf("Extension '%s' not supported\n", extension)
	}

	return img, nil
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

func flipImageVertically(src image.Image) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)

	width := bounds.Max.X
	height := bounds.Max.Y

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dst.Set(x, height-1-y, src.At(x, y))
		}
	}
	return dst
}
