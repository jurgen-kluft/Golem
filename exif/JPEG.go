package ImgMeta

import (
	"encoding/binary"
	"fmt"
	"os"
)

// ============================================== JPEG ==============================================
type exifError struct {
	descr string
}

func (e *exifError) Error() string {
	return fmt.Sprintf("%s", e.descr)
}

// ReadJpeg will read all sections from the image data
func ReadJpeg(fhnd *os.File) (image Image, err error) {
	image = Image{apps: map[string]APP{}}
	reader, n, err := newJpegReader(fhnd)
	if n == 0 || err != nil {
		return
	}

	marker := uint16(0)
	binary.Read(reader, binary.BigEndian, &marker)
	if marker != cSOI {
		return image, &exifError{"Wrong format"}
	}

	//fmt.Println("Reading JPEG APP segments")

	appHeader := make([]byte, 2)
	for true {
		n, err = reader.Read(appHeader)
		if n != len(appHeader) || err != nil {
			break
		}
		if appHeader[0] == 0xFF {
			for appHeader[1] == 0xFF {
				appHeader[1] = reader.ReadByte()
			}

			marker = binary.BigEndian.Uint16(appHeader)
			segment, ok := aSegments[marker]
			if !ok {
				return image, &exifError{"Unidentified APP marker encountered"}
			}
			//fmt.Printf("Encountered marker %s\n", segment.name)

			app, err := segment.reader(marker, reader)
			if err != nil {
				return image, err
			}

			if app == nil {
				break
			}
			fmt.Printf("Registering APP %s\n", app.Name())
			image.apps[app.Name()] = app

		} else {
			// Not a section marker
			marker = binary.BigEndian.Uint16(appHeader)
			return image, &exifError{fmt.Sprintf("Encountered invalid section marker 0x%X", marker)}
		}
	}
	return image, nil
}

type JpegReader struct {
	cursor uint64
	data   []byte
}

func (b *JpegReader) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = b.data[b.cursor]
		b.cursor++
	}
	return len(p), nil
}

func (b *JpegReader) ReadByte() byte {
	v := b.data[b.cursor]
	b.cursor++
	return v
}

func newJpegReader(fhnd *os.File) (reader *JpegReader, n int, err error) {
	stat, err := fhnd.Stat()
	reader = &JpegReader{cursor: 0, data: nil}
	if err != nil {
		return reader, 0, err
	}
	size := stat.Size()
	reader.cursor = 0
	reader.data = make([]byte, size)
	n, err = fhnd.Read(reader.data)
	return
}

func (b *JpegReader) pos() uint64 {
	return b.cursor
}
