package ImgMeta

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

// ============================================== JPEG ==============================================
type exifError struct {
	descr string
}

func (e *exifError) Error() string {
	return fmt.Sprintf("%s", e.descr)
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

			marker = ReadU16(appHeader, binary.BigEndian)
			segment, ok := aSegments[marker]
			if !ok {
				return image, &exifError{"Unidentified APP marker encountered"}
			}
			//fmt.Printf("Encountered marker %s\n", segment.name)

			if segment.action == eEnd {
				//fmt.Printf("Encountered 'end' marked segment %s\n", segment.name)
				break
			}
			if segment.action == eBegin {
				continue
			}

			app, err := segment.reader(marker, reader)
			if err != nil {
				return image, err
			}
			//fmt.Printf("Registering APP %s\n", app.Name())
			image.apps[app.Name()] = app

		} else {
			// Not a section marker
			marker = ReadU16(appHeader, binary.BigEndian)
			return image, &exifError{fmt.Sprintf("Encountered invalid section marker 0x%X", marker)}
		}
	}
	return image, nil
}

// ============================================== EXIF ==============================================

// Image holds both 'Image Data' and 'AP'
type Image struct {
	apps map[string]APP
}

// ReadTagValue reads the value of a tag given as an ID
// Examples:
//             imageWidth := image.ReadTagValue("EXIF", TagImageWidth)
//             imageHeight := image.ReadTagValue("EXIF", TagImageHeight)

func (i Image) ReadTagValue(appname string, tagID uint16) (value interface{}, err error) {
	app, exists := i.apps[appname]
	if !exists {
		fmt.Printf("Image does not have '%s' meta section\n", appname)
		return nil, nil
	}
	value, err = app.ReadValue(tagID)
	return
}

var idJFIF = []byte{'J', 'F', 'I', 'F'}
var idJFXX = []byte{'J', 'F', 'X', 'X'}
var idEXIF = []byte{'E', 'x', 'i', 'f'}
var idXMP = []byte{'h', 't', 't', 'p'}
var idAPP2 = []byte{'I', 'C', 'C', '_'}

type tAPPReader func(uint16, *JpegReader) (APP, error)

type tAPPSegment struct {
	name   string
	marker uint16
	action eAction
	reader tAPPReader
}

func readAPPBlock(marker uint16, reader *JpegReader, extra uint32) (appblock []byte, err error) {
	appLength := uint16(0)
	binary.Read(reader, binary.BigEndian, &appLength)

	size := uint32(appLength) + 2 + extra
	appblock = make([]byte, size)

	// Read the full APP data block into memory
	n, err := reader.Read(appblock[4:])
	if err != nil || n != (len(appblock)-4) {
		return
	}

	WriteU16(marker, appblock, binary.BigEndian)
	WriteU16(appLength, appblock[2:], binary.BigEndian)
	return
}

func readComment(marker uint16, reader *JpegReader) (a APP, err error) {
	//fmt.Print("APP:COMMENT = ")
	var app tAPP
	app.name = "COMMENT"
	app.block, err = readAPPBlock(marker, reader, 0)
	app.offset = 10
	comment := string(app.block[4:])
	fmt.Println(comment)
	return app, nil
}

func readJFIF(app tAPP) (err error) {
	//fmt.Printf("APP:JFIF (length: %d)\n", len(app.block))
	app.name = "JFIF"
	return nil
}

func readJFXX(app tAPP) (err error) {
	//fmt.Printf("APP:JFXX (length: %d)\n", len(app.block))
	app.name = "JFXX"
	return nil
}

func readJF(marker uint16, reader *JpegReader) (a APP, err error) {
	var app tAPP
	app.block, err = readAPPBlock(marker, reader, 0)
	app.offset = 10
	if app.hasIdentifier(idJFIF) {
		return app, readJFIF(app)
	} else if app.hasIdentifier(idJFXX) {
		return app, readJFIF(app)
	}
	return app, &exifError{"APP0 has wrong identifier, should be 'JFIF' or 'JFXX'"}
}

func readTIFF(tiff []byte) (endian binary.ByteOrder, offset uint64, err error) {
	bo := ReadU16(tiff, binary.BigEndian)
	if bo == cINTEL {
		endian = binary.LittleEndian
	} else if bo == cMOTOROLA {
		endian = binary.BigEndian
	} else {
		err = &exifError{"TIFF-header Byte-Order is not matching 'II' or 'MM'"}
		return
	}
	tiffID := ReadU16(tiff[2:], binary.BigEndian)
	if tiffID != 0x002A {
		err = &exifError{fmt.Sprintf("TIFF-header ID is not matching, 0x002A!=0x%X", tiffID)}
		return
	}
	offset = uint64(ReadU32(tiff[4:], binary.BigEndian))
	return
}

func (app tEXIFAPP) read() (err error) {
	//fmt.Printf("APP:EXIF (length: %d)\n", len(app.block))
	return nil
}

func readXMP(app tAPP) (err error) {
	//fmt.Printf("APP:XMP (length: %d)\n", len(app.block))
	return nil
}

// EXIF or XMP
func readAPP1(marker uint16, reader *JpegReader) (a APP, err error) {
	var app tAPP
	app.offset = reader.pos()
	app.block, err = readAPPBlock(marker, reader, 0)
	if app.hasIdentifier(idEXIF) {
		exif := tEXIFAPP{name: "EXIF"}
		exif.block = app.block
		exif.offset = app.offset
		err = exif.read()
		return exif, err
	} else if app.hasIdentifier(idXMP) {
		app.name = "XMP"
		return app, readXMP(app)
	}
	return app, &exifError{"APP1 has wrong identifier, should be 'EXIF' or 'XMP'"}
}

func readICCPROFILE(app tAPP) (err error) {
	//fmt.Printf("APP:ICC_PROFILE (length: %d)\n", len(app.block))
	return nil
}

func readAPP2(marker uint16, reader *JpegReader) (a APP, err error) {
	var app tAPP
	app.block, err = readAPPBlock(marker, reader, 0)
	app.offset = 10
	if app.hasIdentifier(idAPP2) {
		return app, readICCPROFILE(app)
	}
	return app, &exifError{"APP2 has wrong identifier, should be 'ICC_PROFILE'"}
}

func readIgnore(marker uint16, reader *JpegReader) (a APP, err error) {
	var app tAPP
	app.block, err = readAPPBlock(marker, reader, 0)
	app.offset = 10

	app.name = fmt.Sprintf("0x%X", marker)

	// ignore
	//fmt.Printf("APP:[ignore] (length: %d)\n", len(app.block))

	return app, nil
}

// Image Sections
const (
	cSOI = 0xFFD8
	cEOI = 0xFFD9

	cJFIF = 0xFFE0 // APP0, "JFIF\x00" or "JFXX\x00", JFIF
	cEXIF = 0xFFE1 // APP1, "EXIF\x00\x00" or "EXIF\x00\xFF" or "http://ns.adobe.com/xap/1.0/\x00"
	cICC  = 0xFFE2 // APP2, "ICC_PROFILE\x00"
	cMETA = 0xFFE3 // APP3, "META\x00\x00" or "Meta\x00\x00"
	cIPTC = 0xFFED // APP13, "Photoshop 3.0\x00"

	cSOF0  = 0xFFC0 // Start of Frame (baseline JPEG)
	cSOF1  = 0xFFC1 // Start of Frame (baseline JPEG)
	cSOF11 = 0xFFCB // usually unsupported

	cDHT = 0xFFC4 // Huffman Table
	cDAC = 0xFFCC // Define Arithmetic Table, usually unsupported
	cDQT = 0xFFDB // DQT, Quantization table definition
	cSOS = 0xFFDA

	cRST0 = 0xFFD0 // RSTn are used for resync, may be ignored
	cRST7 = 0xFFD7 //
	cTEM  = 0xFF01 // usually causes a decoding error, may be ignored

	cDNL = 0xFFDC // usually unsupported, ignore
	cDRI = 0xFFDD // Define Restart Interval, for details see below
	cDHP = 0xFFDE // ignore (skip)
	cEXP = 0xFFDF // ignore (skip)

	cJPG   = 0xFFC8
	cJPG0  = 0xFFF0 // ignore (skip)
	cJPG13 = 0xFFFD // ignore (skip)

	cCOMMENT = 0xFFFE // Comment, may be ignored
)

type eAction byte

const (
	eBegin eAction = 1
	eEnd   eAction = 2
	eRead  eAction = 3
)

var aSegments = map[uint16]tAPPSegment{

	cSOI:  {name: "SOI", marker: cSOI, action: eBegin, reader: nil},
	cEOI:  {name: "EOI", marker: cEOI, action: eEnd, reader: nil},
	cJFIF: {name: "JFIF", marker: cJFIF, action: eRead, reader: readJF},
	cEXIF: {name: "EXIF", marker: cEXIF, action: eRead, reader: readAPP1},
	cICC:  {name: "ICC", marker: cICC, action: eRead, reader: readAPP2},
	cMETA: {name: "META", marker: cMETA, action: eRead, reader: readIgnore},
	cIPTC: {name: "IPTC", marker: cIPTC, action: eRead, reader: readIgnore},

	cSOF0:     {name: "cSOF0", marker: cSOF0, action: eRead, reader: readIgnore},
	cSOF1:     {name: "cSOF1", marker: cSOF1, action: eRead, reader: readIgnore},
	cSOF1 + 1: {name: "cSOF2", marker: cSOF1 + 1, action: eRead, reader: readIgnore},
	cSOF1 + 2: {name: "cSOF3", marker: cSOF1 + 2, action: eRead, reader: readIgnore},
	cSOF1 + 4: {name: "cSOF5", marker: cSOF1 + 4, action: eRead, reader: readIgnore},
	cSOF1 + 5: {name: "cSOF6", marker: cSOF1 + 5, action: eRead, reader: readIgnore},
	cSOF1 + 6: {name: "cSOF7", marker: cSOF1 + 6, action: eRead, reader: readIgnore},
	cSOF1 + 8: {name: "cSOF9", marker: cSOF1 + 8, action: eRead, reader: readIgnore},
	cSOF1 + 9: {name: "cSOF10", marker: cSOF1 + 9, action: eRead, reader: readIgnore},
	cSOF11:    {name: "cSOF11", marker: cSOF11, action: eRead, reader: readIgnore},

	cDHT: {name: "cDHT", marker: cDHT, action: eRead, reader: readIgnore},
	cDAC: {name: "cDAC", marker: cDAC, action: eRead, reader: readIgnore},
	cDQT: {name: "cDQT", marker: cDQT, action: eRead, reader: readIgnore},
	cSOS: {name: "cSOS", marker: cSOS, action: eEnd, reader: readIgnore},

	cRST0:     {name: "cRST0", marker: cRST0, action: eRead, reader: readIgnore},
	cRST0 + 1: {name: "cRST1", marker: cRST0 + 6, action: eRead, reader: readIgnore},
	cRST0 + 2: {name: "cRST2", marker: cRST0 + 5, action: eRead, reader: readIgnore},
	cRST0 + 3: {name: "cRST3", marker: cRST0 + 4, action: eRead, reader: readIgnore},
	cRST0 + 4: {name: "cRST4", marker: cRST0 + 3, action: eRead, reader: readIgnore},
	cRST0 + 5: {name: "cRST5", marker: cRST0 + 2, action: eRead, reader: readIgnore},
	cRST0 + 6: {name: "cRST6", marker: cRST0 + 1, action: eRead, reader: readIgnore},
	cRST7:     {name: "cRST7", marker: cRST7, action: eRead, reader: readIgnore},
	cTEM:      {name: "cTEM", marker: cTEM, action: eRead, reader: readIgnore},

	cDNL: {name: "cDNL", marker: cDNL, action: eRead, reader: readIgnore},
	cDRI: {name: "cDRI", marker: cDRI, action: eRead, reader: readIgnore},
	cDHP: {name: "cDHP", marker: cDHP, action: eRead, reader: readIgnore},
	cEXP: {name: "cEXP", marker: cEXP, action: eRead, reader: readIgnore},

	cJPG:   {name: "cJPG", marker: cJPG, action: eRead, reader: readIgnore},
	cJPG0:  {name: "cJPG0", marker: cJPG0, action: eRead, reader: readIgnore},
	cJPG13: {name: "cJPG13", marker: cJPG13, action: eRead, reader: readIgnore},

	cCOMMENT: {name: "Comment", marker: cCOMMENT, action: eRead, reader: readComment},
}

// APP represents an APP section of the image file
type APP interface {
	Name() string
	ReadValue(uint16) (interface{}, error)
}

type tAPP struct {
	name   string
	offset uint64           // Offset of this APP in the file
	endian binary.ByteOrder // TIFF-Header, Byte-Order
	block  []byte           // full APP block
}

func (t tAPP) length() uint16 {
	return ReadU16(t.block[2:], t.endian)
}
func (t tAPP) marker() uint16 {
	return ReadU16(t.block[2:], t.endian)
}
func (t tAPP) identifier() (id []byte) {
	id = t.block[4:10]
	return
}
func (t tAPP) hasIdentifier(id []byte) bool {
	aid := t.identifier()
	for i, b := range id {
		if b != aid[i] {
			return false
		}
	}
	return true
}

func (t tAPP) Name() string {
	return t.name
}
func (t tAPP) ReadValue(tagID2Find uint16) (interface{}, error) {
	fmt.Printf("Read value of tag:0x%X in APP:BASIC\n", tagID2Find)
	return int(0), nil
}

type tEXIFAPP struct {
	name   string
	offset uint64           // Offset of this APP in the file
	endian binary.ByteOrder // TIFF-Header, Byte-Order
	block  []byte           // full APP block
}

func (t tEXIFAPP) Name() string {
	return t.name
}

func (t tEXIFAPP) TIFFByteOrder() binary.ByteOrder {
	bo := ReadU16(t.block[10:12], binary.BigEndian)
	if bo == cINTEL {
		return binary.LittleEndian
	}
	return binary.BigEndian
}

func (t tEXIFAPP) TIFFOffsetToIFD0() uint32 {
	endian := t.TIFFByteOrder()
	return ReadU32(t.block[14:18], endian)
}

type ifdOffsetItem struct {
	offset  uint32
	ifdType uint16
}

func (t tEXIFAPP) ReadValue(tagID2Find uint16) (interface{}, error) {
	//fmt.Printf("Read value of tag:0x%X in APP:EXIF\n", tagID2Find)

	tiffOffset := uint32(10)
	ifd0Offset := tiffOffset + t.TIFFOffsetToIFD0()
	endian := t.TIFFByteOrder()

	ifdQueue := []ifdOffsetItem{}
	ifdQueue = append(ifdQueue, ifdOffsetItem{offset: ifd0Offset, ifdType: cIFDZERO})

	for len(ifdQueue) > 0 {
		// Pop the next offset to process
		ifdItem := ifdQueue[len(ifdQueue)-1]
		ifdQueue = ifdQueue[:len(ifdQueue)-1]

		ifd := tExifIFD{offset: ifdItem.offset, appblock: t.block, endian: endian}
		// How many fields does this IFD have ?
		numberOfTags := ifd.NumberOfTags()
		//fmt.Printf("Checking ifd:0x%X at offset:%d with %d fields\n", ifdItem.ifdType, ifdItem.offset, numberOfTags)

		for i := uint32(0); i < numberOfTags; i++ {
			tag := ifd.GetTag(i)
			tagID := tag.TagID()

			//fmt.Printf("Checking tag:0x%X at index:%d\n", tagID, i)
			if tagID == tagID2Find {
				//fmt.Printf("Found tag:0x%X in APP:EXIF at index %d\n", tagID, i)
				return ifd.ReadValue(tag)
			}

			// IFD0, reading the offsets to the other IFD segments
			if ifdItem.ifdType == cIFDZERO && tagID == cIFDEXIF {
				//fmt.Printf("Found ifd:0x%X in APP:EXIF at index %d\n", tagID, i)
				anotherIfdOffset := tiffOffset + tag.valueOrOffset()
				ifdQueue = append(ifdQueue, ifdOffsetItem{offset: anotherIfdOffset, ifdType: cIFDEXIF})
			} else if ifdItem.ifdType == cIFDZERO && tagID == cIFDGPS {
				//fmt.Printf("Found gps:0x%X in APP:EXIF at index %d\n", tagID, i)
				anotherIfdOffset := tiffOffset + tag.valueOrOffset()
				ifdQueue = append(ifdQueue, ifdOffsetItem{offset: anotherIfdOffset, ifdType: cIFDGPS})
			} else if ifdItem.ifdType == cIFDEXIF && tagID == cIFDINTEROP {
				//fmt.Printf("Found iop:0x%X in APP:EXIF at index %d\n", tag, i)
				anotherIfdOffset := tiffOffset + tag.valueOrOffset()
				ifdQueue = append(ifdQueue, ifdOffsetItem{offset: anotherIfdOffset, ifdType: cIFDINTEROP})
			}
		}
	}

	return int(1), nil
}

type tExifIFD struct {
	offset   uint32           // IFD-Offset
	endian   binary.ByteOrder // Endian
	appblock []byte
}

func (ifd tExifIFD) NumberOfTags() uint32 {
	return uint32(ReadU16(ifd.appblock[ifd.offset:], ifd.endian))
}

func (ifd tExifIFD) GetTag(index uint32) tExifTag {
	o := ifd.offset + 2 + (index * 12)
	return tExifTag{appblock: ifd.appblock[o : o+12], endian: ifd.endian}
}

func (ifd tExifIFD) FindTag(id uint16) (tExifTag, bool) {
	n := ifd.NumberOfTags()
	for i := uint32(0); i < n; i++ {
		tag := ifd.GetTag(i)
		if tag.TagID() == id {
			return tag, true
		}
	}
	return tExifTag{appblock: ifd.appblock[0:0], endian: ifd.endian}, false
}

type tExifTag struct {
	endian   binary.ByteOrder
	offset   uint32
	appblock []byte
}

func (tag tExifTag) TagID() uint16 {
	return ReadU16(tag.appblock[tag.offset:], tag.endian)
}
func (tag tExifTag) TypeID() uint16 {
	return ReadU16(tag.appblock[tag.offset+2:], tag.endian)
}
func (tag tExifTag) countOrComponents() int32 {
	return ReadS32(tag.appblock[tag.offset+4:], tag.endian)
}
func (tag tExifTag) valueOrOffset() uint32 {
	return ReadU32(tag.appblock[tag.offset+8:], tag.endian)
}
func (tag tExifTag) valueAsFloat32() float32 {
	bits := binary.LittleEndian.Uint32(tag.appblock[tag.offset+8:])
	float := math.Float32frombits(bits)
	return float
}

type tExifTagFieldType uint16

var aExifTagFieldSize = []int{0, 1, 1, 2, 4, 8, 1, 1, 2, 4, 8, 4, 8}

func getExifTagFieldSize(fieldType tExifTagFieldType) int {
	return aExifTagFieldSize[int(fieldType)]
}

const (
	cUBYTE     = 1
	cASCII     = 2
	cUSHORT    = 3
	cULONG     = 4
	cURATIONAL = 5
	cSBYTE     = 6
	cUNDEFINED = 7
	cSSHORT    = 8
	cSLONG     = 9
	cSRATIONAL = 10
	cFLOAT32   = 11
	cFLOAT64   = 12
)

func (ifd tExifIFD) readValueFromOffset(offset uint32, typeID uint16) (interface{}, error) {
	switch typeID {
	case cFLOAT64:
		bits := ifd.endian.Uint64(ifd.appblock[ifd.offset+offset:])
		float := math.Float64frombits(bits)
		return float, nil
	case cURATIONAL:
		numerator := ifd.endian.Uint32(ifd.appblock[ifd.offset+offset:])
		denominator := ifd.endian.Uint32(ifd.appblock[ifd.offset+offset+4:])
		return float64(numerator) / float64(denominator), nil
	case cSRATIONAL:
		numerator := (ifd.endian.Uint32(ifd.appblock[ifd.offset+offset:]))
		denominator := (ifd.endian.Uint32(ifd.appblock[ifd.offset+offset+4:]))
		return float64(numerator) / float64(denominator), nil
	}
	return int(0), &exifError{"Reading EXIF tag value from offset failed"}
}

func (ifd tExifIFD) ReadValue(tag tExifTag) (interface{}, error) {
	switch tag.TypeID() {
	case cUBYTE:
		return uint8(tag.valueOrOffset()), nil
	case cUSHORT:
		return uint16(tag.valueOrOffset()), nil
	case cULONG:
		return uint32(tag.valueOrOffset()), nil
	case cSBYTE:
		return int8(tag.valueOrOffset()), nil
	case cSSHORT:
		return int16(tag.valueOrOffset()), nil
	case cSLONG:
		return int32(tag.valueOrOffset()), nil
	case cFLOAT32:
		return tag.valueAsFloat32(), nil
	case cFLOAT64:
		return ifd.readValueFromOffset(tag.valueOrOffset(), tag.TypeID())
	case cURATIONAL:
		return ifd.readValueFromOffset(tag.valueOrOffset(), tag.TypeID())
	case cSRATIONAL:
		return ifd.readValueFromOffset(tag.valueOrOffset(), tag.TypeID())
	}
	return int(0), &exifError{"Reading EXIF tag value failed"}
}

const (
	cINTEL    = 0x4949
	cMOTOROLA = 0x4D4D
)

const (
	cIFDZERO    uint16 = 0x0000
	cIFDEXIF    uint16 = 0x8769
	cIFDGPS     uint16 = 0x8825
	cIFDINTEROP uint16 = 0xa005
)

const (
	ExifTagImageWidth                  uint16 = 0x100
	ExifTagImageHeight                 uint16 = 0x101
	ExifTagBitsPerSample               uint16 = 0x102
	ExifTagCompression                 uint16 = 0x103
	ExifTagPhotometricInterpretation   uint16 = 0x106
	ExifTagImageDescription            uint16 = 0x10E
	ExifTagMake                        uint16 = 0x10F
	ExifTagModel                       uint16 = 0x110
	ExifTagStripOffsets                uint16 = 0x111
	ExifTagOrientation                 uint16 = 0x112
	ExifTagSamplesPerPixel             uint16 = 0x115
	ExifTagRowsPerStrip                uint16 = 0x116
	ExifTagStripByteCounts             uint16 = 0x117
	ExifTagXResolution                 uint16 = 0x11A
	ExifTagYResolution                 uint16 = 0x11B
	ExifTagPlanarConfiguration         uint16 = 0x11C
	ExifTagResolutionUnit              uint16 = 0x128
	ExifTagTransferFunction            uint16 = 0x12D
	ExifTagSoftware                    uint16 = 0x131
	ExifTagDateTime                    uint16 = 0x132
	ExifTagArtist                      uint16 = 0x13B
	ExifTagWhitePoint                  uint16 = 0x13E
	ExifTagPrimaryChromaticities       uint16 = 0x13F
	ExifTagJPEGInterchangeFormat       uint16 = 0x201
	ExifTagJPEGInterchangeFormatLength uint16 = 0x202
	ExifTagYCbCrCoefficients           uint16 = 0x211
	ExifTagYCbCrSubSampling            uint16 = 0x212
	ExifTagYCbCrPositioning            uint16 = 0x213
	ExifTagReferenceBlackWhite         uint16 = 0x214
	ExifTagCopyright                   uint16 = 0x8298

	ExifTagExposureTime              uint16 = 0x829A
	ExifTagFNumber                   uint16 = 0x829D
	ExifTagExposureProgram           uint16 = 0x8822
	ExifTagSpectralSensitivity       uint16 = 0x8824
	ExifTagPhotographicSensitivity   uint16 = 0x8827
	ExifTagOECF                      uint16 = 0x8828
	ExifTagSensitivityType           uint16 = 0x8830
	ExifTagStandardOutputSensitivity uint16 = 0x8831
	ExifTagRecommendedExposureIndex  uint16 = 0x8832
	ExifTagISOSpeed                  uint16 = 0x8833
	ExifTagISOSpeedLatitudeyyy       uint16 = 0x8834
	ExifTagISOSpeedLatitudezzz       uint16 = 0x8835
	ExifTagExifVersion               uint16 = 0x9000
	ExifTagDateTimeOriginal          uint16 = 0x9003
	ExifTagDateTimeDigitized         uint16 = 0x9004
	ExifTagComponentsConfiguration   uint16 = 0x9101
	ExifTagCompressedBitsPerPixel    uint16 = 0x9102
	ExifTagShutterSpeedValue         uint16 = 0x9201
	ExifTagApertureValue             uint16 = 0x9202
	ExifTagBrightnessValue           uint16 = 0x9203
	ExifTagExposureBiasValue         uint16 = 0x9204
	ExifTagMaxApertureValue          uint16 = 0x9205
	ExifTagSubjectDistance           uint16 = 0x9206
	ExifTagMeteringMode              uint16 = 0x9207
	ExifTagLightSource               uint16 = 0x9208
	ExifTagFlash                     uint16 = 0x9209
	ExifTagFocalLength               uint16 = 0x920A
	ExifTagSubjectArea               uint16 = 0x9214
	ExifTagMakerNote                 uint16 = 0x927C
	ExifTagUserComment               uint16 = 0x9286
	ExifTagSubsecTime                uint16 = 0x9290
	ExifTagSubsecTimeOriginal        uint16 = 0x9291
	ExifTagSubsecTimeDigitized       uint16 = 0x9292
	ExifTagFlashpixVersion           uint16 = 0xA000
	ExifTagColorSpace                uint16 = 0xA001
	ExifTagPixelXDimension           uint16 = 0xA002
	ExifTagPixelYDimension           uint16 = 0xA003
	ExifTagRelatedSoundFile          uint16 = 0xA004
	ExifTagFlashEnergy               uint16 = 0xA20B
	ExifTagSpatialFrequencyResponse  uint16 = 0xA20C
	ExifTagFocalPlaneXResolution     uint16 = 0xA20E
	ExifTagFocalPlaneYResolution     uint16 = 0xA20F
	ExifTagFocalPlaneResolutionUnit  uint16 = 0xA210
	ExifTagSubjectLocation           uint16 = 0xA214
	ExifTagExposureIndex             uint16 = 0xA215
	ExifTagSensingMethod             uint16 = 0xA217
	ExifTagFileSource                uint16 = 0xA300
	ExifTagSceneType                 uint16 = 0xA301
	ExifTagCFAPattern                uint16 = 0xA302
	ExifTagCustomRendered            uint16 = 0xA401
	ExifTagExposureMode              uint16 = 0xA402
	ExifTagWhiteBalance              uint16 = 0xA403
	ExifTagDigitalZoomRatio          uint16 = 0xA404
	ExifTagFocalLengthIn35mmFilm     uint16 = 0xA405
	ExifTagSceneCaptureType          uint16 = 0xA406
	ExifTagGainControl               uint16 = 0xA407
	ExifTagContrast                  uint16 = 0xA408
	ExifTagSaturation                uint16 = 0xA409
	ExifTagSharpness                 uint16 = 0xA40A
	ExifTagDeviceSettingDescription  uint16 = 0xA40B
	ExifTagSubjectDistanceRange      uint16 = 0xA40C
	ExifTagImageUniqueID             uint16 = 0xA420
	ExifTagCameraOwnerName           uint16 = 0xA430
	ExifTagBodySerialNumber          uint16 = 0xA431
	ExifTagLensSpecification         uint16 = 0xA432
	ExifTagLensMake                  uint16 = 0xA433
	ExifTagLensModel                 uint16 = 0xA434
	ExifTagLensSerialNumber          uint16 = 0xA435

	ExifGpsTagGPSVersionID         uint16 = 0x0
	ExifGpsTagGPSLatitudeRef       uint16 = 0x1
	ExifGpsTagGPSLatitude          uint16 = 0x2
	ExifGpsTagGPSLongitudeRef      uint16 = 0x3
	ExifGpsTagGPSLongitude         uint16 = 0x4
	ExifGpsTagGPSAltitudeRef       uint16 = 0x5
	ExifGpsTagGPSAltitude          uint16 = 0x6
	ExifGpsTagGPSTimestamp         uint16 = 0x7
	ExifGpsTagGPSSatellites        uint16 = 0x8
	ExifGpsTagGPSStatus            uint16 = 0x9
	ExifGpsTagGPSMeasureMode       uint16 = 0xA
	ExifGpsTagGPSDOP               uint16 = 0xB
	ExifGpsTagGPSSpeedRef          uint16 = 0xC
	ExifGpsTagGPSSpeed             uint16 = 0xD
	ExifGpsTagGPSTrackRef          uint16 = 0xE
	ExifGpsTagGPSTrack             uint16 = 0xF
	ExifGpsTagGPSImgDirectionRef   uint16 = 0x10
	ExifGpsTagGPSImgDirection      uint16 = 0x11
	ExifGpsTagGPSMapDatum          uint16 = 0x12
	ExifGpsTagGPSDestLatitudeRef   uint16 = 0x13
	ExifGpsTagGPSDestLatitude      uint16 = 0x14
	ExifGpsTagGPSDestLongitudeRef  uint16 = 0x15
	ExifGpsTagGPSDestLongitude     uint16 = 0x16
	ExifGpsTagGPSDestBearingRef    uint16 = 0x17
	ExifGpsTagGPSDestBearing       uint16 = 0x18
	ExifGpsTagGPSDestDistanceRef   uint16 = 0x19
	ExifGpsTagGPSDestDistance      uint16 = 0x1A
	ExifGpsTagGPSProcessingMethod  uint16 = 0x1B
	ExifGpsTagGPSAreaInformation   uint16 = 0x1C
	ExifGpsTagGPSDateStamp         uint16 = 0x1D
	ExifGpsTagGPSDifferential      uint16 = 0x1E
	ExifGpsTagGPSHPositioningError uint16 = 0x1F

	ExifXpTagXPTitle    uint16 = 0x9c9b
	ExifXpTagXPComment  uint16 = 0x9c9c
	ExifXpTagXPAuthor   uint16 = 0x9c9d
	ExifXpTagXPKeywords uint16 = 0x9c9e
	ExifXpTagXPSubject  uint16 = 0x9c9f
)

type tExifTagDescr struct {
	tag  uint16
	id   uint16
	name string
}

var aExifTagDescr = map[uint16]tExifTagDescr{
	// Primary tags
	ExifTagImageWidth:                  {tag: cIFDZERO, name: "ImageWidth", id: ExifTagImageWidth},
	ExifTagImageHeight:                 {tag: cIFDZERO, name: "ImageLength", id: ExifTagImageHeight},
	ExifTagBitsPerSample:               {tag: cIFDZERO, name: "BitsPerSample", id: ExifTagBitsPerSample},
	ExifTagCompression:                 {tag: cIFDZERO, name: "Compression", id: ExifTagCompression},
	ExifTagPhotometricInterpretation:   {tag: cIFDZERO, name: "PhotometricInterpretation", id: ExifTagPhotometricInterpretation},
	ExifTagImageDescription:            {tag: cIFDZERO, name: "ImageDescription", id: ExifTagImageDescription},
	ExifTagMake:                        {tag: cIFDZERO, name: "Make", id: ExifTagMake},
	ExifTagModel:                       {tag: cIFDZERO, name: "Model", id: ExifTagModel},
	ExifTagStripOffsets:                {tag: cIFDZERO, name: "StripOffsets", id: ExifTagStripOffsets},
	ExifTagOrientation:                 {tag: cIFDZERO, name: "Orientation", id: ExifTagOrientation},
	ExifTagSamplesPerPixel:             {tag: cIFDZERO, name: "SamplesPerPixel", id: ExifTagSamplesPerPixel},
	ExifTagRowsPerStrip:                {tag: cIFDZERO, name: "RowsPerStrip", id: ExifTagRowsPerStrip},
	ExifTagStripByteCounts:             {tag: cIFDZERO, name: "StripByteCounts", id: ExifTagStripByteCounts},
	ExifTagXResolution:                 {tag: cIFDZERO, name: "XResolution", id: ExifTagXResolution},
	ExifTagYResolution:                 {tag: cIFDZERO, name: "YResolution", id: ExifTagYResolution},
	ExifTagPlanarConfiguration:         {tag: cIFDZERO, name: "PlanarConfiguration", id: ExifTagPlanarConfiguration},
	ExifTagResolutionUnit:              {tag: cIFDZERO, name: "ResolutionUnit", id: ExifTagResolutionUnit},
	ExifTagTransferFunction:            {tag: cIFDZERO, name: "TransferFunction", id: ExifTagTransferFunction},
	ExifTagSoftware:                    {tag: cIFDZERO, name: "Software", id: ExifTagSoftware},
	ExifTagDateTime:                    {tag: cIFDZERO, name: "DateTime", id: ExifTagDateTime},
	ExifTagArtist:                      {tag: cIFDZERO, name: "Artist", id: ExifTagArtist},
	ExifTagWhitePoint:                  {tag: cIFDZERO, name: "WhitePoint", id: ExifTagWhitePoint},
	ExifTagPrimaryChromaticities:       {tag: cIFDZERO, name: "PrimaryChromaticities", id: ExifTagPrimaryChromaticities},
	ExifTagJPEGInterchangeFormat:       {tag: cIFDZERO, name: "JPEGInterchangeFormat", id: ExifTagJPEGInterchangeFormat},
	ExifTagJPEGInterchangeFormatLength: {tag: cIFDZERO, name: "JPEGInterchangeFormatLength", id: ExifTagJPEGInterchangeFormatLength},
	ExifTagYCbCrCoefficients:           {tag: cIFDZERO, name: "YCbCrCoefficients", id: ExifTagYCbCrCoefficients},
	ExifTagYCbCrSubSampling:            {tag: cIFDZERO, name: "YCbCrSubSampling", id: ExifTagYCbCrSubSampling},
	ExifTagYCbCrPositioning:            {tag: cIFDZERO, name: "YCbCrPositioning", id: ExifTagYCbCrPositioning},
	ExifTagReferenceBlackWhite:         {tag: cIFDZERO, name: "ReferenceBlackWhite", id: ExifTagReferenceBlackWhite},
	ExifTagCopyright:                   {tag: cIFDZERO, name: "Copyright", id: ExifTagCopyright},

	// EXIF tags
	ExifTagExposureTime:              {tag: cIFDEXIF, name: "ExposureTime", id: ExifTagExposureTime},
	ExifTagFNumber:                   {tag: cIFDEXIF, name: "FNumber", id: ExifTagFNumber},
	ExifTagExposureProgram:           {tag: cIFDEXIF, name: "ExposureProgram", id: ExifTagExposureProgram},
	ExifTagSpectralSensitivity:       {tag: cIFDEXIF, name: "SpectralSensitivity", id: ExifTagSpectralSensitivity},
	ExifTagPhotographicSensitivity:   {tag: cIFDEXIF, name: "PhotographicSensitivity", id: ExifTagPhotographicSensitivity},
	ExifTagOECF:                      {tag: cIFDEXIF, name: "OECF", id: ExifTagOECF},
	ExifTagSensitivityType:           {tag: cIFDEXIF, name: "SensitivityType", id: ExifTagSensitivityType},
	ExifTagStandardOutputSensitivity: {tag: cIFDEXIF, name: "StandardOutputSensitivity", id: ExifTagStandardOutputSensitivity},
	ExifTagRecommendedExposureIndex:  {tag: cIFDEXIF, name: "RecommendedExposureIndex", id: ExifTagRecommendedExposureIndex},
	ExifTagISOSpeed:                  {tag: cIFDEXIF, name: "ISOSpeed", id: ExifTagISOSpeed},
	ExifTagISOSpeedLatitudeyyy:       {tag: cIFDEXIF, name: "ISOSpeedLatitudeyyy", id: ExifTagISOSpeedLatitudeyyy},
	ExifTagISOSpeedLatitudezzz:       {tag: cIFDEXIF, name: "ISOSpeedLatitudezzz", id: ExifTagISOSpeedLatitudezzz},
	ExifTagExifVersion:               {tag: cIFDEXIF, name: "ExifVersion", id: ExifTagExifVersion},
	ExifTagDateTimeOriginal:          {tag: cIFDEXIF, name: "DateTimeOriginal", id: ExifTagDateTimeOriginal},
	ExifTagDateTimeDigitized:         {tag: cIFDEXIF, name: "DateTimeDigitized", id: ExifTagDateTimeDigitized},
	ExifTagComponentsConfiguration:   {tag: cIFDEXIF, name: "ComponentsConfiguration", id: ExifTagComponentsConfiguration},
	ExifTagCompressedBitsPerPixel:    {tag: cIFDEXIF, name: "CompressedBitsPerPixel", id: ExifTagCompressedBitsPerPixel},
	ExifTagShutterSpeedValue:         {tag: cIFDEXIF, name: "ShutterSpeedValue", id: ExifTagShutterSpeedValue},
	ExifTagApertureValue:             {tag: cIFDEXIF, name: "ApertureValue", id: ExifTagApertureValue},
	ExifTagBrightnessValue:           {tag: cIFDEXIF, name: "BrightnessValue", id: ExifTagBrightnessValue},
	ExifTagExposureBiasValue:         {tag: cIFDEXIF, name: "ExposureBiasValue", id: ExifTagExposureBiasValue},
	ExifTagMaxApertureValue:          {tag: cIFDEXIF, name: "MaxApertureValue", id: ExifTagMaxApertureValue},
	ExifTagSubjectDistance:           {tag: cIFDEXIF, name: "SubjectDistance", id: ExifTagSubjectDistance},
	ExifTagMeteringMode:              {tag: cIFDEXIF, name: "MeteringMode", id: ExifTagMeteringMode},
	ExifTagLightSource:               {tag: cIFDEXIF, name: "LightSource", id: ExifTagLightSource},
	ExifTagFlash:                     {tag: cIFDEXIF, name: "Flash", id: ExifTagFlash},
	ExifTagFocalLength:               {tag: cIFDEXIF, name: "FocalLength", id: ExifTagFocalLength},
	ExifTagSubjectArea:               {tag: cIFDEXIF, name: "SubjectArea", id: ExifTagSubjectArea},
	ExifTagMakerNote:                 {tag: cIFDEXIF, name: "MakerNote", id: ExifTagMakerNote},
	ExifTagUserComment:               {tag: cIFDEXIF, name: "UserComment", id: ExifTagUserComment},
	ExifTagSubsecTime:                {tag: cIFDEXIF, name: "SubsecTime", id: ExifTagSubsecTime},
	ExifTagSubsecTimeOriginal:        {tag: cIFDEXIF, name: "SubsecTimeOriginal", id: ExifTagSubsecTimeOriginal},
	ExifTagSubsecTimeDigitized:       {tag: cIFDEXIF, name: "SubsecTimeDigitized", id: ExifTagSubsecTimeDigitized},
	ExifTagFlashpixVersion:           {tag: cIFDEXIF, name: "FlashpixVersion", id: ExifTagFlashpixVersion},
	ExifTagColorSpace:                {tag: cIFDEXIF, name: "ColorSpace", id: ExifTagColorSpace},
	ExifTagPixelXDimension:           {tag: cIFDEXIF, name: "PixelXDimension", id: ExifTagPixelXDimension},
	ExifTagPixelYDimension:           {tag: cIFDEXIF, name: "PixelYDimension", id: ExifTagPixelYDimension},
	ExifTagRelatedSoundFile:          {tag: cIFDEXIF, name: "RelatedSoundFile", id: ExifTagRelatedSoundFile},
	ExifTagFlashEnergy:               {tag: cIFDEXIF, name: "FlashEnergy", id: ExifTagFlashEnergy},
	ExifTagSpatialFrequencyResponse:  {tag: cIFDEXIF, name: "SpatialFrequencyResponse", id: ExifTagSpatialFrequencyResponse},
	ExifTagFocalPlaneXResolution:     {tag: cIFDEXIF, name: "FocalPlaneXResolution", id: ExifTagFocalPlaneXResolution},
	ExifTagFocalPlaneYResolution:     {tag: cIFDEXIF, name: "FocalPlaneYResolution", id: ExifTagFocalPlaneYResolution},
	ExifTagFocalPlaneResolutionUnit:  {tag: cIFDEXIF, name: "FocalPlaneResolutionUnit", id: ExifTagFocalPlaneResolutionUnit},
	ExifTagSubjectLocation:           {tag: cIFDEXIF, name: "SubjectLocation", id: ExifTagSubjectLocation},
	ExifTagExposureIndex:             {tag: cIFDEXIF, name: "ExposureIndex", id: ExifTagExposureIndex},
	ExifTagSensingMethod:             {tag: cIFDEXIF, name: "SensingMethod", id: ExifTagSensingMethod},
	ExifTagFileSource:                {tag: cIFDEXIF, name: "FileSource", id: ExifTagFileSource},
	ExifTagSceneType:                 {tag: cIFDEXIF, name: "SceneType", id: ExifTagSceneType},
	ExifTagCFAPattern:                {tag: cIFDEXIF, name: "CFAPattern", id: ExifTagCFAPattern},
	ExifTagCustomRendered:            {tag: cIFDEXIF, name: "CustomRendered", id: ExifTagCustomRendered},
	ExifTagExposureMode:              {tag: cIFDEXIF, name: "ExposureMode", id: ExifTagExposureMode},
	ExifTagWhiteBalance:              {tag: cIFDEXIF, name: "WhiteBalance", id: ExifTagWhiteBalance},
	ExifTagDigitalZoomRatio:          {tag: cIFDEXIF, name: "DigitalZoomRatio", id: ExifTagDigitalZoomRatio},
	ExifTagFocalLengthIn35mmFilm:     {tag: cIFDEXIF, name: "FocalLengthIn35mmFilm", id: ExifTagFocalLengthIn35mmFilm},
	ExifTagSceneCaptureType:          {tag: cIFDEXIF, name: "SceneCaptureType", id: ExifTagSceneCaptureType},
	ExifTagGainControl:               {tag: cIFDEXIF, name: "GainControl", id: ExifTagGainControl},
	ExifTagContrast:                  {tag: cIFDEXIF, name: "Contrast", id: ExifTagContrast},
	ExifTagSaturation:                {tag: cIFDEXIF, name: "Saturation", id: ExifTagSaturation},
	ExifTagSharpness:                 {tag: cIFDEXIF, name: "Sharpness", id: ExifTagSharpness},
	ExifTagDeviceSettingDescription:  {tag: cIFDEXIF, name: "DeviceSettingDescription", id: ExifTagDeviceSettingDescription},
	ExifTagSubjectDistanceRange:      {tag: cIFDEXIF, name: "SubjectDistanceRange", id: ExifTagSubjectDistanceRange},
	ExifTagImageUniqueID:             {tag: cIFDEXIF, name: "ImageUniqueID", id: ExifTagImageUniqueID},
	ExifTagCameraOwnerName:           {tag: cIFDEXIF, name: "CameraOwnerName", id: ExifTagCameraOwnerName},
	ExifTagBodySerialNumber:          {tag: cIFDEXIF, name: "BodySerialNumber", id: ExifTagBodySerialNumber},
	ExifTagLensSpecification:         {tag: cIFDEXIF, name: "LensSpecification", id: ExifTagLensSpecification},
	ExifTagLensMake:                  {tag: cIFDEXIF, name: "LensMake", id: ExifTagLensMake},
	ExifTagLensModel:                 {tag: cIFDEXIF, name: "LensModel", id: ExifTagLensModel},
	ExifTagLensSerialNumber:          {tag: cIFDEXIF, name: "LensSerialNumber", id: ExifTagLensSerialNumber},

	// GPS tags
	ExifGpsTagGPSVersionID:         {tag: cIFDGPS, name: "GPSVersionID", id: ExifGpsTagGPSVersionID},
	ExifGpsTagGPSLatitudeRef:       {tag: cIFDGPS, name: "GPSLatitudeRef", id: ExifGpsTagGPSLatitudeRef},
	ExifGpsTagGPSLatitude:          {tag: cIFDGPS, name: "GPSLatitude", id: ExifGpsTagGPSLatitude},
	ExifGpsTagGPSLongitudeRef:      {tag: cIFDGPS, name: "GPSLongitudeRef", id: ExifGpsTagGPSLongitudeRef},
	ExifGpsTagGPSLongitude:         {tag: cIFDGPS, name: "GPSLongitude", id: ExifGpsTagGPSLongitude},
	ExifGpsTagGPSAltitudeRef:       {tag: cIFDGPS, name: "GPSAltitudeRef", id: ExifGpsTagGPSAltitudeRef},
	ExifGpsTagGPSAltitude:          {tag: cIFDGPS, name: "GPSAltitude", id: ExifGpsTagGPSAltitude},
	ExifGpsTagGPSTimestamp:         {tag: cIFDGPS, name: "GPSTimestamp", id: ExifGpsTagGPSTimestamp},
	ExifGpsTagGPSSatellites:        {tag: cIFDGPS, name: "GPSSatellites", id: ExifGpsTagGPSSatellites},
	ExifGpsTagGPSStatus:            {tag: cIFDGPS, name: "GPSStatus", id: ExifGpsTagGPSStatus},
	ExifGpsTagGPSMeasureMode:       {tag: cIFDGPS, name: "GPSMeasureMode", id: ExifGpsTagGPSMeasureMode},
	ExifGpsTagGPSDOP:               {tag: cIFDGPS, name: "GPSDOP", id: ExifGpsTagGPSDOP},
	ExifGpsTagGPSSpeedRef:          {tag: cIFDGPS, name: "GPSSpeedRef", id: ExifGpsTagGPSSpeedRef},
	ExifGpsTagGPSSpeed:             {tag: cIFDGPS, name: "GPSSpeed", id: ExifGpsTagGPSSpeed},
	ExifGpsTagGPSTrackRef:          {tag: cIFDGPS, name: "GPSTrackRef", id: ExifGpsTagGPSTrackRef},
	ExifGpsTagGPSTrack:             {tag: cIFDGPS, name: "GPSTrack", id: ExifGpsTagGPSTrack},
	ExifGpsTagGPSImgDirectionRef:   {tag: cIFDGPS, name: "GPSImgDirectionRef", id: ExifGpsTagGPSImgDirectionRef},
	ExifGpsTagGPSImgDirection:      {tag: cIFDGPS, name: "GPSImgDirection", id: ExifGpsTagGPSImgDirection},
	ExifGpsTagGPSMapDatum:          {tag: cIFDGPS, name: "GPSMapDatum", id: ExifGpsTagGPSMapDatum},
	ExifGpsTagGPSDestLatitudeRef:   {tag: cIFDGPS, name: "GPSDestLatitudeRef", id: ExifGpsTagGPSDestLatitudeRef},
	ExifGpsTagGPSDestLatitude:      {tag: cIFDGPS, name: "GPSDestLatitude", id: ExifGpsTagGPSDestLatitude},
	ExifGpsTagGPSDestLongitudeRef:  {tag: cIFDGPS, name: "GPSDestLongitudeRef", id: ExifGpsTagGPSDestLongitudeRef},
	ExifGpsTagGPSDestLongitude:     {tag: cIFDGPS, name: "GPSDestLongitude", id: ExifGpsTagGPSDestLongitude},
	ExifGpsTagGPSDestBearingRef:    {tag: cIFDGPS, name: "GPSDestBearingRef", id: ExifGpsTagGPSDestBearingRef},
	ExifGpsTagGPSDestBearing:       {tag: cIFDGPS, name: "GPSDestBearing", id: ExifGpsTagGPSDestBearing},
	ExifGpsTagGPSDestDistanceRef:   {tag: cIFDGPS, name: "GPSDestDistanceRef", id: ExifGpsTagGPSDestDistanceRef},
	ExifGpsTagGPSDestDistance:      {tag: cIFDGPS, name: "GPSDestDistance", id: ExifGpsTagGPSDestDistance},
	ExifGpsTagGPSProcessingMethod:  {tag: cIFDGPS, name: "GPSProcessingMethod", id: ExifGpsTagGPSProcessingMethod},
	ExifGpsTagGPSAreaInformation:   {tag: cIFDGPS, name: "GPSAreaInformation", id: ExifGpsTagGPSAreaInformation},
	ExifGpsTagGPSDateStamp:         {tag: cIFDGPS, name: "GPSDateStamp", id: ExifGpsTagGPSDateStamp},
	ExifGpsTagGPSDifferential:      {tag: cIFDGPS, name: "GPSDifferential", id: ExifGpsTagGPSDifferential},
	ExifGpsTagGPSHPositioningError: {tag: cIFDGPS, name: "GPSHPositioningError", id: ExifGpsTagGPSHPositioningError},

	// Microsoft Windows metadata. Non-standard, but ubiquitous
	ExifXpTagXPTitle:    {tag: cIFDZERO, name: "XPTitle", id: ExifXpTagXPTitle},
	ExifXpTagXPComment:  {tag: cIFDZERO, name: "XPComment", id: ExifXpTagXPComment},
	ExifXpTagXPAuthor:   {tag: cIFDZERO, name: "XPAuthor", id: ExifXpTagXPAuthor},
	ExifXpTagXPKeywords: {tag: cIFDZERO, name: "XPKeywords", id: ExifXpTagXPKeywords},
	ExifXpTagXPSubject:  {tag: cIFDZERO, name: "XPSubject", id: ExifXpTagXPSubject},
}

const (
	cExposureProgram      = 0x00010000
	cMeteringMode         = 0x00020000
	cLightSource          = 0x00030000
	cFlash                = 0x00040000
	cSensingMethod        = 0x00050000
	cSceneCaptureType     = 0x00060000
	cSceneType            = 0x00070000
	cCustomRendered       = 0x00080000
	cWhiteBalance         = 0x00090000
	cGainControl          = 0x000A0000
	cContrast             = 0x000B0000
	cSaturation           = 0x000C0000
	cSharpness            = 0x000D0000
	cSubjectDistanceRange = 0x000E0000
	cFileSource           = 0x000F0000
	cComponents           = 0x00100000
)

var aExifStringEnums = map[int]string{
	cExposureProgram + 0: "Not defined",
	cExposureProgram + 1: "Manual",
	cExposureProgram + 2: "Normal program",
	cExposureProgram + 3: "Aperture priority",
	cExposureProgram + 4: "Shutter priority",
	cExposureProgram + 5: "Creative program",
	cExposureProgram + 6: "Action program",
	cExposureProgram + 7: "Portrait mode",
	cExposureProgram + 8: "Landscape mode",

	cMeteringMode + 0:   "Unknown",
	cMeteringMode + 1:   "Average",
	cMeteringMode + 2:   "CenterWeightedAverage",
	cMeteringMode + 3:   "Spot",
	cMeteringMode + 4:   "MultiSpot",
	cMeteringMode + 5:   "Pattern",
	cMeteringMode + 6:   "Partial",
	cMeteringMode + 255: "Other",

	cLightSource + 0:   "Unknown",
	cLightSource + 1:   "Daylight",
	cLightSource + 2:   "Fluorescent",
	cLightSource + 3:   "Tungsten (incandescent light)",
	cLightSource + 4:   "Flash",
	cLightSource + 9:   "Fine weather",
	cLightSource + 10:  "Cloudy weather",
	cLightSource + 11:  "Shade",
	cLightSource + 12:  "Daylight fluorescent (D 5700 - 7100K)",
	cLightSource + 13:  "Day white fluorescent (N 4600 - 5400K)",
	cLightSource + 14:  "Cool white fluorescent (W 3900 - 4500K)",
	cLightSource + 15:  "White fluorescent (WW 3200 - 3700K)",
	cLightSource + 17:  "Standard light A",
	cLightSource + 18:  "Standard light B",
	cLightSource + 19:  "Standard light C",
	cLightSource + 20:  "D55",
	cLightSource + 21:  "D65",
	cLightSource + 22:  "D75",
	cLightSource + 23:  "D50",
	cLightSource + 24:  "ISO studio tungsten",
	cLightSource + 255: "Other",

	cFlash + 0x0000: "Flash did not fire",
	cFlash + 0x0001: "Flash fired",
	cFlash + 0x0005: "Strobe return light not detected",
	cFlash + 0x0007: "Strobe return light detected",
	cFlash + 0x0009: "Flash fired, compulsory flash mode",
	cFlash + 0x000D: "Flash fired, compulsory flash mode, return light not detected",
	cFlash + 0x000F: "Flash fired, compulsory flash mode, return light detected",
	cFlash + 0x0010: "Flash did not fire, compulsory flash mode",
	cFlash + 0x0018: "Flash did not fire, auto mode",
	cFlash + 0x0019: "Flash fired, auto mode",
	cFlash + 0x001D: "Flash fired, auto mode, return light not detected",
	cFlash + 0x001F: "Flash fired, auto mode, return light detected",
	cFlash + 0x0020: "No flash function",
	cFlash + 0x0041: "Flash fired, red-eye reduction mode",
	cFlash + 0x0045: "Flash fired, red-eye reduction mode, return light not detected",
	cFlash + 0x0047: "Flash fired, red-eye reduction mode, return light detected",
	cFlash + 0x0049: "Flash fired, compulsory flash mode, red-eye reduction mode",
	cFlash + 0x004D: "Flash fired, compulsory flash mode, red-eye reduction mode, return light not detected",
	cFlash + 0x004F: "Flash fired, compulsory flash mode, red-eye reduction mode, return light detected",
	cFlash + 0x0059: "Flash fired, auto mode, red-eye reduction mode",
	cFlash + 0x005D: "Flash fired, auto mode, return light not detected, red-eye reduction mode",
	cFlash + 0x005F: "Flash fired, auto mode, return light detected, red-eye reduction mode",

	cSensingMethod + 1: "Not defined",
	cSensingMethod + 2: "One-chip color area sensor",
	cSensingMethod + 3: "Two-chip color area sensor",
	cSensingMethod + 4: "Three-chip color area sensor",
	cSensingMethod + 5: "Color sequential area sensor",
	cSensingMethod + 7: "Trilinear sensor",
	cSensingMethod + 8: "Color sequential linear sensor",

	cSceneCaptureType + 0: "Standard",
	cSceneCaptureType + 1: "Landscape",
	cSceneCaptureType + 2: "Portrait",
	cSceneCaptureType + 3: "Night scene",

	cSceneType + 1: "Directly photographed",

	cCustomRendered + 0: "Normal process",
	cCustomRendered + 1: "Custom process",

	cWhiteBalance + 0: "Auto white balance",
	cWhiteBalance + 1: "Manual white balance",

	cGainControl + 0: "None",
	cGainControl + 1: "Low gain up",
	cGainControl + 2: "High gain up",
	cGainControl + 3: "Low gain down",
	cGainControl + 4: "High gain down",

	cContrast + 0: "Normal",
	cContrast + 1: "Soft",
	cContrast + 2: "Hard",

	cSaturation + 0: "Normal",
	cSaturation + 1: "Low saturation",
	cSaturation + 2: "High saturation",

	cSharpness + 0: "Normal",
	cSharpness + 1: "Soft",
	cSharpness + 2: "Hard",

	cSubjectDistanceRange + 0: "Unknown",
	cSubjectDistanceRange + 1: "Macro",
	cSubjectDistanceRange + 2: "Close view",
	cSubjectDistanceRange + 3: "Distant view",

	cFileSource + 3: "DSC",

	cComponents + 0: "",
	cComponents + 1: "Y",
	cComponents + 2: "Cb",
	cComponents + 3: "Cr",
	cComponents + 4: "R",
	cComponents + 5: "G",
	cComponents + 6: "B",
}

// ReadU16 does read a unsigned short from the start of the byte slice
func ReadU16(slice []byte, endian binary.ByteOrder) (value uint16) {
	if endian == binary.BigEndian {
		value = uint16(slice[0])<<8 | uint16(slice[1])
	} else {
		value = uint16(slice[1])<<8 | uint16(slice[0])
	}
	return
}

// ReadU32 does read a unsigned 32-bit integer from the start of the byte slice
func ReadU32(slice []byte, endian binary.ByteOrder) uint32 {
	if endian == binary.BigEndian {
		return uint32(slice[0])<<24 | uint32(slice[1])<<16 | uint32(slice[2])<<8 | uint32(slice[3])
	}
	return uint32(slice[3])<<24 | uint32(slice[2])<<16 | uint32(slice[1])<<8 | uint32(slice[0])
}

// ReadS32 does read a signed 32-bit integer from the start of the byte slice
func ReadS32(slice []byte, endian binary.ByteOrder) int32 {
	if endian == binary.BigEndian {
		return int32(slice[0])<<24 | int32(slice[1])<<16 | int32(slice[2])<<8 | int32(slice[3])
	}
	return int32(slice[3])<<24 | int32(slice[2])<<16 | int32(slice[1])<<8 | int32(slice[0])
}

// WriteU16 does read a unsigned short from the start of the byte slice
func WriteU16(value uint16, slice []byte, endian binary.ByteOrder) {
	if endian == binary.BigEndian {
		slice[0], slice[1] = byte(value>>8), byte(value)
	} else {
		slice[1], slice[0] = byte(value>>8), byte(value)
	}
}
