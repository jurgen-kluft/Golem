package main

import (
	"fmt"
	"github.com/jurgen-kluft/golem/exif"
	"io"
	"os"
)

type byteReader struct {
	io.Reader

	cursor uint64
	data   []byte
}

func (b *byteReader) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = b.data[b.cursor]
		b.cursor++
	}
	return len(p), nil
}

func (b *byteReader) ReadInAll(fhnd *os.File) (n int, err error) {
	stat, err := fhnd.Stat()
	if err != nil {
		return 0, err
	}
	size := stat.Size()
	b.data = make([]byte, size)
	n, err = fhnd.Read(b.data)
	return
}

func (b *byteReader) Pos() uint64 {
	return b.cursor
}

func main() {
	fhnd, err := os.Open("test.jpg")
	if err != nil {
		return
	}
	reader := &byteReader{}
	_, err = reader.ReadInAll(fhnd)

	image, err := EXIF.ReadJpeg(reader)
	if err != nil {
		pos := reader.Pos()
		fmt.Printf("File position at %d (hex = 0x%X)\n", pos, pos)
		fmt.Println(err.Error())
		return
	}

	basicInfo := EXIF.GetBasicInfo(image)
	fmt.Printf("Image: width:%d, height:%d\n", basicInfo.Width, basicInfo.Height)
}
