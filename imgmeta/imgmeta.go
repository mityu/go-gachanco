package imgmeta

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
)

type MetaData struct {
	Width  uint
	Height uint
	Type   string
}

func Parse(path string) (MetaData, error) {
	f, err := os.Open(path)
	if err != nil {
		return MetaData{}, err
	}
	defer f.Close()

	decoded, imgType, err := image.Decode(f)
	if err != nil {
		return MetaData{}, err
	}

	bounds := decoded.Bounds()

	return MetaData{
		Width:  uint(bounds.Dx()),
		Height: uint(bounds.Dy()),
		Type:   imgType,
	}, nil
}
