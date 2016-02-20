package ImgMeta

import (
	"encoding/binary"
	"fmt"
)

var idJFIF = []byte{'J', 'F', 'I', 'F', 0}
var idJFXX = []byte{'J', 'F', 'X', 'X', 0}
var idEXIF = []byte{'E', 'x', 'i', 'f', 0, 0}
var idXMP = []byte{'h', 't', 't', 'p', ':', '/', '/', 'n', 's', '.', 'a', 'd', 'o', 'b', 'e', '.', 'c', 'o', 'm', '/', 'x', 'a', 'p', '/', '1', '.', '0', '/', 0}
var idAPP2 = []byte{'I', 'C', 'C', '_', 'P', 'R', 'O', 'F', 'I', 'L', 'E', 0}
var idIPTC = []byte{'P', 'h', 'o', 't', 'o', 's', 'h', 'o', 'p', ' ', '3', '.', '0', 0}

// TIFF Header - Byte Order
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

func fAPPReadBlock(marker uint16, reader *JpegReader, extra uint32) (appblock []byte, err error) {
	appLength := uint16(0)
	binary.Read(reader, binary.BigEndian, &appLength)

	size := uint32(appLength) + 2 + extra
	appblock = make([]byte, size)

	// Read the full APP data block into memory
	n, err := reader.Read(appblock[4:])
	if err != nil || n != (len(appblock)-4) {
		return
	}

	binary.BigEndian.PutUint16(appblock, marker)
	binary.BigEndian.PutUint16(appblock[2:], appLength)
	return
}

func fAPPReadComment(marker uint16, reader *JpegReader) (a APP, err error) {
	//fmt.Print("APP:COMMENT = ")
	app := &tAPP{offset: 0, endian: binary.BigEndian}
	app.block, err = fAPPReadBlock(marker, reader, 0)
	comment := string(app.block[4:])
	fmt.Println(comment)
	return app, nil
}

func fAPPReadJFIF(app *tAPP) (err error) {
	//fmt.Printf("APP:JFIF (length: %d)\n", len(app.block))
	return nil
}

func fAPPReadJFXX(app *tAPP) (err error) {
	//fmt.Printf("APP:JFXX (length: %d)\n", len(app.block))
	return nil
}

func fAPPReadJF(marker uint16, reader *JpegReader) (a APP, err error) {
	app := &tAPP{offset: 10, endian: binary.BigEndian}
	app.block, err = fAPPReadBlock(marker, reader, 0)
	if app.HasID(idJFIF) {
		return app, fAPPReadJFIF(app)
	} else if app.HasID(idJFXX) {
		return app, fAPPReadJFIF(app)
	}
	return app, &exifError{"APP0 has wrong identifier, should be 'JFIF' or 'JFXX'"}
}

// EXIF or XMP
func fAPPReadAPP1(marker uint16, reader *JpegReader) (a APP, err error) {
	app := &tAPP{offset: reader.pos(), endian: binary.BigEndian}
	app.block, err = fAPPReadBlock(marker, reader, 0)
	if app.HasID(idEXIF) {
		exif := &tEXIFAPP{block: app.block, offset: app.offset}
		return exif, err
	} else if app.HasID(idXMP) {
		return app, nil
	}
	return app, &exifError{"APP1 has wrong identifier, should be 'EXIF' or 'XMP'"}
}

func fAPPReadICCPROFILE(app *tAPP) (err error) {
	//fmt.Printf("APP:ICC_PROFILE (length: %d)\n", len(app.block))
	return nil
}

func fAPPReadAPP2(marker uint16, reader *JpegReader) (a APP, err error) {
	app := &tAPP{offset: 10, endian: binary.BigEndian}
	app.block, err = fAPPReadBlock(marker, reader, 0)
	if app.HasID(idAPP2) {
		return app, fAPPReadICCPROFILE(app)
	}
	return app, &exifError{"APP2 has wrong identifier, should be 'ICC_PROFILE'"}
}

func fAPPReadIPTC(marker uint16, reader *JpegReader) (a APP, err error) {
	app := &tIPTCAPP{offset: 10, endian: binary.BigEndian}
	app.block, err = fAPPReadBlock(marker, reader, 0)
	if app.HasID(idIPTC) {
		//fmt.Printf("APP:IPTC (length: %d)\n", len(app.block))
		return app, nil
	}
	return app, &exifError{"APP13 has wrong identifier, should be 'Photoshop 3.0\000'"}
}

func fAPPReadIgnore(marker uint16, reader *JpegReader) (a APP, err error) {
	app := &tAPP{offset: 10, endian: binary.BigEndian}
	app.block, err = fAPPReadBlock(marker, reader, 0)
	return app, nil
}

func fAPPEnd(marker uint16, reader *JpegReader) (a APP, err error) {
	return nil, nil
}

type tAPPReader func(uint16, *JpegReader) (APP, error)

type tAPPSegment struct {
	name   string
	marker uint16
	reader tAPPReader
}

var aSegments = map[uint16]tAPPSegment{

	cSOI:  {name: "SOI", marker: cSOI, reader: nil},
	cEOI:  {name: "EOI", marker: cEOI, reader: nil},
	cJFIF: {name: "JFIF", marker: cJFIF, reader: fAPPReadJF},
	cEXIF: {name: "EXIF", marker: cEXIF, reader: fAPPReadAPP1},
	cICC:  {name: "ICC", marker: cICC, reader: fAPPReadAPP2},
	cMETA: {name: "META", marker: cMETA, reader: fAPPReadIgnore},
	cIPTC: {name: "IPTC", marker: cIPTC, reader: fAPPReadIPTC},

	cSOF0:     {name: "cSOF0", marker: cSOF0, reader: fAPPReadIgnore},
	cSOF1:     {name: "cSOF1", marker: cSOF1, reader: fAPPReadIgnore},
	cSOF1 + 1: {name: "cSOF2", marker: cSOF1 + 1, reader: fAPPReadIgnore},
	cSOF1 + 2: {name: "cSOF3", marker: cSOF1 + 2, reader: fAPPReadIgnore},
	cSOF1 + 4: {name: "cSOF5", marker: cSOF1 + 4, reader: fAPPReadIgnore},
	cSOF1 + 5: {name: "cSOF6", marker: cSOF1 + 5, reader: fAPPReadIgnore},
	cSOF1 + 6: {name: "cSOF7", marker: cSOF1 + 6, reader: fAPPReadIgnore},
	cSOF1 + 8: {name: "cSOF9", marker: cSOF1 + 8, reader: fAPPReadIgnore},
	cSOF1 + 9: {name: "cSOF10", marker: cSOF1 + 9, reader: fAPPReadIgnore},
	cSOF11:    {name: "cSOF11", marker: cSOF11, reader: fAPPReadIgnore},

	cDHT: {name: "cDHT", marker: cDHT, reader: fAPPReadIgnore},
	cDAC: {name: "cDAC", marker: cDAC, reader: fAPPReadIgnore},
	cDQT: {name: "cDQT", marker: cDQT, reader: fAPPReadIgnore},
	cSOS: {name: "cSOS", marker: cSOS, reader: fAPPEnd},

	cRST0:     {name: "cRST0", marker: cRST0, reader: fAPPReadIgnore},
	cRST0 + 1: {name: "cRST1", marker: cRST0 + 6, reader: fAPPReadIgnore},
	cRST0 + 2: {name: "cRST2", marker: cRST0 + 5, reader: fAPPReadIgnore},
	cRST0 + 3: {name: "cRST3", marker: cRST0 + 4, reader: fAPPReadIgnore},
	cRST0 + 4: {name: "cRST4", marker: cRST0 + 3, reader: fAPPReadIgnore},
	cRST0 + 5: {name: "cRST5", marker: cRST0 + 2, reader: fAPPReadIgnore},
	cRST0 + 6: {name: "cRST6", marker: cRST0 + 1, reader: fAPPReadIgnore},
	cRST7:     {name: "cRST7", marker: cRST7, reader: fAPPReadIgnore},
	cTEM:      {name: "cTEM", marker: cTEM, reader: fAPPReadIgnore},

	cDNL: {name: "cDNL", marker: cDNL, reader: fAPPReadIgnore},
	cDRI: {name: "cDRI", marker: cDRI, reader: fAPPReadIgnore},
	cDHP: {name: "cDHP", marker: cDHP, reader: fAPPReadIgnore},
	cEXP: {name: "cEXP", marker: cEXP, reader: fAPPReadIgnore},

	cJPG:   {name: "cJPG", marker: cJPG, reader: fAPPReadIgnore},
	cJPG0:  {name: "cJPG0", marker: cJPG0, reader: fAPPReadIgnore},
	cJPG13: {name: "cJPG13", marker: cJPG13, reader: fAPPReadIgnore},

	cCOMMENT: {name: "Comment", marker: cCOMMENT, reader: fAPPReadComment},
}

// APP represents an APP section of the image file
type APP interface {
	Name() string
	Marker() uint16
	Length() uint16
	ID([]byte) []byte
	HasID([]byte) bool
	ReadValue(uint16) (interface{}, error)
}

type tAPP struct {
	offset uint64           // Offset of this APP in the file
	endian binary.ByteOrder // TIFF-Header, Byte-Order
	block  []byte           // full APP block
}

func (t tAPP) Name() string {
	marker := t.Marker()
	seg, ok := aSegments[marker]
	if !ok {
		return fmt.Sprintf("0x%X", t.Marker())
	}
	return seg.name
}
func (t tAPP) Marker() uint16 {
	if t.block == nil || len(t.block) < 2 {
		return 0
	}
	return t.endian.Uint16(t.block)
}
func (t tAPP) Length() uint16 {
	return t.endian.Uint16(t.block[2:])
}
func (t tAPP) ID(cid []byte) (id []byte) {
	id = t.block[4 : 4+len(cid)]
	return
}
func (t tAPP) HasID(cid []byte) bool {
	id := t.block[4 : 4+len(cid)]
	for i, b := range id {
		if b != cid[i] {
			return false
		}
	}
	return true
}

func (t tAPP) ReadValue(tagID2Find uint16) (interface{}, error) {
	fmt.Printf("Read value of tag:0x%X in APP:BASIC\n", tagID2Find)
	return int(0), nil
}
