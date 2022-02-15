package imgmeta

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	TypeJPEG = "JPEG"
	TypePNG  = "PNG"
	TypeGIF  = "GIF"
	TypeBMP  = "BMP"
)

type MetaData struct {
	Width  uint
	Height uint
	Type   string
}

func (a MetaData) EqualTo(b MetaData) bool {
	return a.Width == b.Width && a.Height == b.Height && a.Type == b.Type
}

func Parse(path string) (MetaData, error) {
	f, err := os.Open(path)
	if err != nil {
		return MetaData{}, err
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return MetaData{}, err
	}

	// NOTE: fpdf may not support BMP.
	parsers := []metaDataParser{
		pngParser{},
		jpegParser{},
		gifParser{},
		bmpParser{},
	}

	var m MetaData
	for _, p := range parsers {
		m, err = p.parse(bytes.NewReader(b))
		if err != nil {
			return MetaData{}, err
		} else if !m.EqualTo(MetaData{}) {
			return m, nil
		}
	}
	return MetaData{}, errors.New("Not a valid image: " + path)
}

func bigEndianUint(b []byte) uint {
	return uint(b[0])<<8*3 + uint(b[1])<<8*2 + uint(b[2])<<8 + uint(b[3])
}

func bigEndianInt(b []byte) int {
	return int(b[0])<<8*3 + int(b[1])<<8*2 + int(b[2])<<8 + int(b[3])
}

func littleEndianUint(b []byte) uint {
	return uint(b[3])<<8*3 + uint(b[2])<<8*2 + uint(b[1])<<8 + uint(b[0])
}

func littleEndianInt(b []byte) int {
	return int(b[3])<<8*3 + int(b[2])<<8*2 + int(b[1])<<8 + int(b[0])
}

type metaDataParser interface {
	parse(*bytes.Reader) (MetaData, error)
}

type jpegParser struct{}

func (_ jpegParser) parse(r *bytes.Reader) (MetaData, error) {
	buf := make([]byte, 2, 2)
	if _, err := r.Read(buf); err != nil {
		return MetaData{}, err
	} else if !(buf[0] == 0xff && buf[1] == 0xd8) {
		return MetaData{}, nil
	}

	for {
		if _, err := r.Read(buf); err != nil {
			return MetaData{}, err
		}
		for buf[0] != 0xff {
			var err error
			buf[0] = buf[1]
			buf[1], err = r.ReadByte()
			if err != nil {
				return MetaData{}, err
			}
		}
		marker := buf[1]
		if marker == 0 {
			continue // Treat "0xff00" as extraneous data
		}
		for marker == 0xff {
			var err error
			marker, err = r.ReadByte()
			if err != nil {
				return MetaData{}, err
			}
		}
		if marker == 0xd9 {
			// End of image.
			return MetaData{}, nil
		} else if 0xd0 <= marker && marker <= 0xd7 {
			// Restart marker
			continue
		}

		if _, err := r.Read(buf); err != nil {
			return MetaData{}, nil
		}

		chunkLen := int(buf[0])<<8 + int(buf[1]) - 2
		if chunkLen < 0 {
			return MetaData{}, errors.New("Shoft segment length")
		}

		if 0xc0 <= marker && marker <= 0xc2 {
			// Parse metadata
			r.ReadByte() // Throw away precision data

			m := MetaData{Type: TypeJPEG}
			if _, err := r.Read(buf); err != nil {
				return MetaData{}, err
			}
			m.Height = uint(int(buf[0])<<8 + int(buf[1]))

			if _, err := r.Read(buf); err != nil {
				return MetaData{}, err
			}
			m.Width = uint(int(buf[0])<<8 + int(buf[1]))

			return m, nil
		} else if _, err := r.Seek(int64(chunkLen), io.SeekCurrent); err != nil {
			return MetaData{}, err
		}
	}
}

type pngParser struct{}

func (_ pngParser) parse(r *bytes.Reader) (MetaData, error) {
	// Check header
	seg := make([]byte, 8, 8)
	if _, err := r.Read(seg); err != nil {
		return MetaData{}, err
	}
	if s := string(seg); s != "\x89PNG\r\n\x1a\n" {
		return MetaData{}, nil
	}

	// Parse metadata
	// TODO: Verify checksum?
	buf := make([]byte, 4, 4)
	for {
		if _, err := r.Read(buf); err != nil {
			return MetaData{}, err
		}
		chunkLen := bigEndianUint(buf)
		if _, err := r.Read(buf); err != nil {
			return MetaData{}, err
		}
		switch string(buf) {
		case "IHDR":
			if chunkLen != 13 {
				return MetaData{}, errors.New(
					"Bad IHDR chunk length: " + fmt.Sprint(chunkLen))
			}
			b := make([]byte, 4, 4)
			m := MetaData{Type: TypePNG}
			if _, err := r.Read(b); err != nil {
				return MetaData{}, err
			}
			m.Width = bigEndianUint(b)
			if _, err := r.Read(b); err != nil {
				return MetaData{}, err
			}
			m.Height = bigEndianUint(b)
			return m, nil
		case "IEND":
			return MetaData{}, nil
		default:
			if _, err := r.Seek(int64(chunkLen), io.SeekCurrent); err != nil {
				return MetaData{}, err
			}
			break
		}
		r.Seek(4, io.SeekCurrent) // Throw away checksum.
	}
}

type gifParser struct{}

func (_ gifParser) parse(r *bytes.Reader) (MetaData, error) {
	// Check header
	seg := make([]byte, 6, 6)
	if _, err := r.Read(seg); err != nil {
		return MetaData{}, err
	}
	if s := string(seg); !(s == "GIF87a" || s == "GIF89a") {
		return MetaData{}, nil
	}

	// Parse metadata
	w := make([]byte, 2, 2)
	if n, err := r.Read(w); err != nil {
		return MetaData{}, err
	} else if n < len(w) {
	}

	h := make([]byte, 2, 2)
	if n, err := r.Read(h); err != nil {
	} else if n < len(h) {
	}

	m := MetaData{
		Width:  uint(w[0]) + uint(w[1])<<8,
		Height: uint(h[0]) + uint(h[1])<<8,
		Type:   TypeGIF,
	}

	return m, nil
}

type bmpParser struct{}

func (_ bmpParser) parse(r *bytes.Reader) (MetaData, error) {
	// Check header
	seg := make([]byte, 2, 2)
	if _, err := r.Read(seg); err != nil {
		return MetaData{}, err
	} else if string(seg) != "BM" {
		return MetaData{}, nil
	}

	// Skip unnecessary information
	if _, err := r.Seek(16, io.SeekCurrent); err != nil {
		return MetaData{}, err
	}

	// Parse metadata
	m := MetaData{Type: TypeBMP}
	buf := make([]byte, 4, 4)
	if _, err := r.Read(buf); err != nil {
		return MetaData{}, nil
	}
	m.Width = littleEndianUint(buf)
	if _, err := r.Read(buf); err != nil {
		return MetaData{}, nil
	}
	h := littleEndianInt(buf)
	if h < 0 {
		h *= -1
	}
	m.Height = uint(h)
	return m, nil
}
