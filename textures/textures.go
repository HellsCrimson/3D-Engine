package textures

import (
	"github.com/go-gl/gl/v4.6-core/gl"
	"neilpa.me/go-stbi"
)

type Texture struct {
	Width  int32
	Height int32
	Data   []byte
}

func Load(name string, isTransparent *bool) (uint32, error) {
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

	for i := 3; i < len(texture.Data); i += 4 { // Alpha channel is every 4th byte
		if texture.Data[i] < 255 {
			*isTransparent = true
			break
		}
	}

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, texture.Width, texture.Height, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(texture.Data))
	gl.GenerateMipmap(gl.TEXTURE_2D)

	return textureId, nil
}

func LoadCubemap(path string) (uint32, error) {
	var textureId uint32
	gl.GenTextures(1, &textureId)
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, textureId)

	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_R, gl.CLAMP_TO_EDGE)

	var face_name string
	for i := uint32(0); i < 6; i++ {
		switch i {
		case 0:
			face_name = "right"
		case 1:
			face_name = "left"
		case 2:
			face_name = "top"
		case 3:
			face_name = "bottom"
		case 4:
			face_name = "front"
		case 5:
			face_name = "back"
		}
		texture, err := getImage(path + "/" + face_name + ".jpg")
		if err != nil {
			return 0, err
		}
		gl.TexImage2D(gl.TEXTURE_CUBE_MAP_POSITIVE_X+i, 0, gl.RGBA, texture.Width, texture.Height, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(texture.Data))
	}

	return textureId, nil
}

func getImage(name string) (*Texture, error) {
	img, err := stbi.Load(name)
	if err != nil {
		return nil, err
	}

	return &Texture{
		Width:  int32(img.Rect.Dx()),
		Height: int32(img.Rect.Dy()),
		Data:   img.Pix,
	}, nil
}
