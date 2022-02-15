package imgmeta

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
)

func readImage(path string) (*bytes.Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if b, err := io.ReadAll(f); err != nil {
		return nil, err
	} else {
		return bytes.NewReader(b), nil
	}
}

func testParser(t *testing.T, p metaDataParser, path string, expected MetaData) {
	r, err := readImage(path)
	if err != nil {
		t.Fatal(err)
	}

	m, err := p.parse(r)
	if err != nil {
		t.Fatal(err)
	}

	if !m.EqualTo(expected) {
		t.Fatal(fmt.Sprintf("\nExpected:\n\t%v\nGot:\n\t%v\n", m, expected))
	}
}

func TestParseGif(t *testing.T) {
	p := gifParser{}
	testParser(t, p, "./testdata/image1.gif", MetaData{
		Width:  20,
		Height: 40,
		Type:   TypeGIF,
	})

	testParser(t, p, "./testdata/image1.png", MetaData{})
	testParser(t, p, "./testdata/image1.jpg", MetaData{})
}

func TestParsePng(t *testing.T) {
	p := pngParser{}
	testParser(t, p, "./testdata/image1.png", MetaData{
		Width:  20,
		Height: 40,
		Type:   TypePNG,
	})

	testParser(t, p, "./testdata/image1.gif", MetaData{})
	testParser(t, p, "./testdata/image1.jpg", MetaData{})
}

func TestParseJpeg(t *testing.T) {
	p := jpegParser{}
	testParser(t, p, "./testdata/image1.jpg", MetaData{
		Width:  20,
		Height: 40,
		Type:   TypeJPEG,
	})

	testParser(t, p, "./testdata/image1.png", MetaData{})
	testParser(t, p, "./testdata/image1.gif", MetaData{})
}

func TestParseBmp(t *testing.T) {
	p := bmpParser{}
	testParser(t, p, "./testdata/image1.bmp", MetaData{
		Width:  20,
		Height: 40,
		Type:   TypeBMP,
	})

	testParser(t, p, "./testdata/image1.gif", MetaData{})
	testParser(t, p, "./testdata/image1.jpg", MetaData{})
	testParser(t, p, "./testdata/image1.png", MetaData{})
}
