package ImgMeta

import (
	"fmt"
)

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
