package EXIF

import (
	"encoding/binary"
	"fmt"
	"io"
)

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

// ============================================== JPEG ==============================================
type exifError struct {
	descr string
}

func (e *exifError) Error() string {
	return fmt.Sprintf("%s", e.descr)
}

// ReadJpeg will read all sections from the image data
func ReadJpeg(reader io.Reader) (image Image, err error) {

	marker := uint16(0)
	binary.Read(reader, binary.BigEndian, &marker)
	if marker != cSOI {
		return image, &exifError{"Wrong format"}
	}

	appHeader := make([]byte, 4)
	for true {
		reader.Read(appHeader)
		marker = ReadU16(appHeader, binary.BigEndian)
		if (marker & 0xFF00) == 0xFF00 {
			if marker == cEOI {
				break
			}

			applen := ReadU16(appHeader[2:], binary.BigEndian)
			appSection := make([]byte, applen+2)

			// Read the full APP data block into memory
			reader.Read(appSection[4:])

			WriteU16(marker, appSection, binary.BigEndian)
			WriteU16(applen, appSection[2:], binary.BigEndian)

			app, err := readEXIF(appSection)
			if err != nil {
				return image, &exifError{"Error encountered when reading an APP"}
			}
			image.apps = append(image.apps, app)

		} else {
			// Not a section marker
			return image, &exifError{"Encountered invalid section marker"}
		}
	}
	return image, nil
}

func readTIFF(tiff []byte) (endian binary.ByteOrder, offset uint32, err error) {
	tiffID := ReadU16(tiff, binary.BigEndian)
	if tiffID != 0x002A {
		err = &exifError{"TIFF-header ID is not matching (0x00, 0x2A)"}
		return
	}
	bo := ReadU16(tiff, binary.BigEndian)
	if bo == cINTEL {
		endian = binary.LittleEndian
	} else if bo == cMOTOROLA {
		endian = binary.BigEndian
	} else {
		err = &exifError{"TIFF-header Byte-Order is not matching 'II' or 'MM'"}
		return
	}
	offset = ReadU32(tiff, binary.BigEndian)
	return
}

// ============================================== EXIF ==============================================

// Image holds both 'Image Data' and 'AP'
type Image struct {
	apps []tAPP
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
	cSOF2  = 0xFFC2 // usually unsupported
	cSOF3  = 0xFFC3 // usually unsupported
	cSOF5  = 0xFFC5 // usually unsupported
	cSOF6  = 0xFFC6 // usually unsupported
	cSOF7  = 0xFFC7 // usually unsupported
	cSOF9  = 0xFFC9 // usually unsupported, for arithmetic coding
	cSOF10 = 0xFFCA // usually unsupported
	cSOF11 = 0xFFCB // usually unsupported

	cDHT = 0xFFC4 // Huffman Table
	cDAC = 0xFFCC // Define Arithmetic Table, usually unsupported
	cDQT = 0xFFDB // DQT, Quantization table definition
	cSOS = 0xFFDA

	cRST0 = 0xFFD0 // RSTn are used for resync, may be ignored
	cRST1 = 0xFFD1 //
	cRST2 = 0xFFD2 //
	cRST3 = 0xFFD3 //
	cRST4 = 0xFFD4 //
	cRST5 = 0xFFD5 //
	cRST6 = 0xFFD6 //
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

var idJFIF = []byte{'J', 'F', 'I', 'F'}
var idJFXX = []byte{'J', 'F', 'X', 'X'}
var idEXIF = []byte{'E', 'x', 'i', 'f'}
var idXMP = []byte{'h', 't', 't', 'p'}

type eAction byte

const (
	eIgnore eAction = 0
	eBegin  eAction = 1
	eEnd    eAction = 2
	eRead   eAction = 3
)

type tSegmentReader struct {
	id     []byte
	reader func([]byte) (tAPP, error)
}

type tSegment struct {
	name    string
	marker  uint16
	action  eAction
	readers []tSegmentReader
}

func readComment(appsection []byte) (app tAPP, err error) {
	return
}

func readJFIF(appsection []byte) (app tAPP, err error) {
	return
}
func readJFXX(appsection []byte) (app tAPP, err error) {
	return
}

func readEXIF(appsection []byte) (app tAPP, err error) {
	app.slice = appsection
	app.offset = 10
	app.endian, app.offset, err = readTIFF(appsection[10:18])
	return
}

func readXMP(appsection []byte) (app tAPP, err error) {
	return
}

var aSegments = map[uint16]tSegment{

	cSOI:  {name: "Start of Image", marker: cSOI, action: eBegin, readers: nil},
	cEOI:  {name: "End of Image", marker: cEOI, action: eEnd, readers: nil},
	cJFIF: {name: "JFIF application segment", marker: cJFIF, action: eRead, readers: []tSegmentReader{{id: idJFIF, reader: readJFIF}, {id: idJFXX, reader: readJFXX}}},
	cEXIF: {name: "EXIF application segment", marker: cEXIF, action: eRead, readers: []tSegmentReader{{id: idEXIF, reader: readEXIF}, {id: idXMP, reader: readXMP}}},
	cICC:  {},
	cMETA: {},
	cIPTC: {},

	cSOF0:  {},
	cSOF1:  {},
	cSOF2:  {},
	cSOF3:  {},
	cSOF5:  {},
	cSOF6:  {},
	cSOF7:  {},
	cSOF9:  {},
	cSOF10: {},
	cSOF11: {},

	cDHT: {},
	cDAC: {},
	cDQT: {},
	cSOS: {},

	cRST0: {},
	cRST1: {},
	cRST2: {},
	cRST3: {},
	cRST4: {},
	cRST5: {},
	cRST6: {},
	cRST7: {},
	cTEM:  {},

	cDNL: {},
	cDRI: {},
	cDHP: {},
	cEXP: {},

	cJPG:   {},
	cJPG0:  {},
	cJPG13: {},

	cCOMMENT: {name: "Comment", marker: cCOMMENT, action: eRead, readers: []tSegmentReader{{id: []byte{}, reader: readComment}}},
}

type tData struct {
	offset uint64 // Offset in file
	size   uint64 // Image data size (including begin and end marker)
	data   []byte
}

type tAPP struct {
	offset uint32           // TIFF-Offset
	endian binary.ByteOrder // TIFF-Header, Byte-Order
	slice  []byte           // full APP block
}

func (t tAPP) length() uint16 {
	return ReadU16(t.slice[2:], t.endian)
}
func (t tAPP) marker() uint16 {
	return ReadU16(t.slice[2:], t.endian)
}
func (t tAPP) identifier() (id []byte) {
	id = t.slice[4:10]
	return
}

type tExifIFD struct {
	offset uint             // TIFF-Offset
	endian binary.ByteOrder // TIFF-Header, Byte-Order
	slice  []byte
}

func (t tExifIFD) numberOfExifTags() (value uint16) {
	value = ReadU16(t.slice, t.endian)
	return
}

func (t tExifIFD) offsetToIFD0() (value uint32) {
	o := 2 + t.numberOfExifTags()*12
	value = ReadU32(t.slice[o:], t.endian)
	return
}
func (t tExifIFD) getExifTag(index int) tExifTag {
	o := 2 + (index * 12)
	return tExifTag{slice: t.slice[o : o+12], endian: t.endian}
}

type tExifTag struct {
	endian binary.ByteOrder
	slice  []byte
}

func (t tExifTag) Tag() uint16 {
	return ReadU16(t.slice, t.endian)
}
func (t tExifTag) Type() uint16 {
	return ReadU16(t.slice[2:], t.endian)
}
func (t tExifTag) Count() int32 {
	return ReadS32(t.slice[4:], t.endian)
}
func (t tExifTag) ValueOffset() int32 {
	return ReadS32(t.slice[8:], t.endian)
}

type tExifTagFieldType uint16

var aExifTagFieldSize = []int{0, 1, 1, 2, 4, 8, 1, 1, 2, 4, 8, 4, 8}

func getExifTagFieldSize(fieldType tExifTagFieldType) int {
	return aExifTagFieldSize[int(fieldType)]
}

const (
	cUBYTE     tExifTagFieldType = 1
	cASCII     tExifTagFieldType = 2
	cUSHORT    tExifTagFieldType = 3
	cULONG     tExifTagFieldType = 4
	cURATIONAL tExifTagFieldType = 5
	cSBYTE     tExifTagFieldType = 6
	cUNDEFINED tExifTagFieldType = 7
	cSSHORT    tExifTagFieldType = 8
	cSLONG     tExifTagFieldType = 9
	cSRATIONAL tExifTagFieldType = 10
	cFLOAT32   tExifTagFieldType = 11
	cFLOAT64   tExifTagFieldType = 12
)

const (
	cINTEL    = 0x4949
	cMOTOROLA = 0x4D4D
)

type tExifTagType uint16

const (
	cIFD0TT tExifTagType = 0xA005
	cEXIFTT tExifTagType = 0x8769
	cGPSTT  tExifTagType = 0x8825
)

type tExifTagDescr struct {
	tag  tExifTagType
	id   uint16
	name string
}

var aExifTagDescr = []tExifTagDescr{
	// primary tags
	{tag: cIFD0TT, name: "ImageWidth", id: 0x100},
	{tag: cIFD0TT, name: "ImageLength", id: 0x101},
	{tag: cIFD0TT, name: "BitsPerSample", id: 0x102},
	{tag: cIFD0TT, name: "Compression", id: 0x103},
	{tag: cIFD0TT, name: "PhotometricInterpretation", id: 0x106},
	{tag: cIFD0TT, name: "ImageDescription", id: 0x10E},
	{tag: cIFD0TT, name: "Make", id: 0x10F},
	{tag: cIFD0TT, name: "Model", id: 0x110},
	{tag: cIFD0TT, name: "StripOffsets", id: 0x111},
	{tag: cIFD0TT, name: "Orientation", id: 0x112},
	{tag: cIFD0TT, name: "SamplesPerPixel", id: 0x115},
	{tag: cIFD0TT, name: "RowsPerStrip", id: 0x116},
	{tag: cIFD0TT, name: "StripByteCounts", id: 0x117},
	{tag: cIFD0TT, name: "XResolution", id: 0x11A},
	{tag: cIFD0TT, name: "YResolution", id: 0x11B},
	{tag: cIFD0TT, name: "PlanarConfiguration", id: 0x11C},
	{tag: cIFD0TT, name: "ResolutionUnit", id: 0x128},
	{tag: cIFD0TT, name: "TransferFunction", id: 0x12D},
	{tag: cIFD0TT, name: "Software", id: 0x131},
	{tag: cIFD0TT, name: "DateTime", id: 0x132},
	{tag: cIFD0TT, name: "Artist", id: 0x13B},
	{tag: cIFD0TT, name: "WhitePoint", id: 0x13E},
	{tag: cIFD0TT, name: "PrimaryChromaticities", id: 0x13F},
	{tag: cIFD0TT, name: "JPEGInterchangeFormat", id: 0x201},
	{tag: cIFD0TT, name: "JPEGInterchangeFormatLength", id: 0x202},
	{tag: cIFD0TT, name: "YCbCrCoefficients", id: 0x211},
	{tag: cIFD0TT, name: "YCbCrSubSampling", id: 0x212},
	{tag: cIFD0TT, name: "YCbCrPositioning", id: 0x213},
	{tag: cIFD0TT, name: "ReferenceBlackWhite", id: 0x214},
	{tag: cIFD0TT, name: "Copyright", id: 0x8298},

	// EXIF tags
	{tag: cEXIFTT, name: "ExposureTime", id: 0x829A},
	{tag: cEXIFTT, name: "FNumber", id: 0x829D},
	{tag: cEXIFTT, name: "ExposureProgram", id: 0x8822},
	{tag: cEXIFTT, name: "SpectralSensitivity", id: 0x8824},
	{tag: cEXIFTT, name: "PhotographicSensitivity", id: 0x8827},
	{tag: cEXIFTT, name: "OECF", id: 0x8828},
	{tag: cEXIFTT, name: "SensitivityType", id: 0x8830},
	{tag: cEXIFTT, name: "StandardOutputSensitivity", id: 0x8831},
	{tag: cEXIFTT, name: "RecommendedExposureIndex", id: 0x8832},
	{tag: cEXIFTT, name: "ISOSpeed", id: 0x8833},
	{tag: cEXIFTT, name: "ISOSpeedLatitudeyyy", id: 0x8834},
	{tag: cEXIFTT, name: "ISOSpeedLatitudezzz", id: 0x8835},

	{tag: cEXIFTT, name: "ExifVersion", id: 0x9000},
	{tag: cEXIFTT, name: "DateTimeOriginal", id: 0x9003},
	{tag: cEXIFTT, name: "DateTimeDigitized", id: 0x9004},
	{tag: cEXIFTT, name: "ComponentsConfiguration", id: 0x9101},
	{tag: cEXIFTT, name: "CompressedBitsPerPixel", id: 0x9102},
	{tag: cEXIFTT, name: "ShutterSpeedValue", id: 0x9201},
	{tag: cEXIFTT, name: "ApertureValue", id: 0x9202},
	{tag: cEXIFTT, name: "BrightnessValue", id: 0x9203},
	{tag: cEXIFTT, name: "ExposureBiasValue", id: 0x9204},
	{tag: cEXIFTT, name: "MaxApertureValue", id: 0x9205},
	{tag: cEXIFTT, name: "SubjectDistance", id: 0x9206},
	{tag: cEXIFTT, name: "MeteringMode", id: 0x9207},
	{tag: cEXIFTT, name: "LightSource", id: 0x9208},
	{tag: cEXIFTT, name: "Flash", id: 0x9209},
	{tag: cEXIFTT, name: "FocalLength", id: 0x920A},
	{tag: cEXIFTT, name: "SubjectArea", id: 0x9214},
	{tag: cEXIFTT, name: "MakerNote", id: 0x927C},
	{tag: cEXIFTT, name: "UserComment", id: 0x9286},
	{tag: cEXIFTT, name: "SubsecTime", id: 0x9290},
	{tag: cEXIFTT, name: "SubsecTimeOriginal", id: 0x9291},
	{tag: cEXIFTT, name: "SubsecTimeDigitized", id: 0x9292},
	{tag: cEXIFTT, name: "FlashpixVersion", id: 0xA000},
	{tag: cEXIFTT, name: "ColorSpace", id: 0xA001},
	{tag: cEXIFTT, name: "PixelXDimension", id: 0xA002},
	{tag: cEXIFTT, name: "PixelYDimension", id: 0xA003},
	{tag: cEXIFTT, name: "RelatedSoundFile", id: 0xA004},
	{tag: cEXIFTT, name: "FlashEnergy", id: 0xA20B},
	{tag: cEXIFTT, name: "SpatialFrequencyResponse", id: 0xA20C},
	{tag: cEXIFTT, name: "FocalPlaneXResolution", id: 0xA20E},
	{tag: cEXIFTT, name: "FocalPlaneYResolution", id: 0xA20F},
	{tag: cEXIFTT, name: "FocalPlaneResolutionUnit", id: 0xA210},
	{tag: cEXIFTT, name: "SubjectLocation", id: 0xA214},
	{tag: cEXIFTT, name: "ExposureIndex", id: 0xA215},
	{tag: cEXIFTT, name: "SensingMethod", id: 0xA217},
	{tag: cEXIFTT, name: "FileSource", id: 0xA300},
	{tag: cEXIFTT, name: "SceneType", id: 0xA301},
	{tag: cEXIFTT, name: "CFAPattern", id: 0xA302},
	{tag: cEXIFTT, name: "CustomRendered", id: 0xA401},
	{tag: cEXIFTT, name: "ExposureMode", id: 0xA402},
	{tag: cEXIFTT, name: "WhiteBalance", id: 0xA403},
	{tag: cEXIFTT, name: "DigitalZoomRatio", id: 0xA404},
	{tag: cEXIFTT, name: "FocalLengthIn35mmFilm", id: 0xA405},
	{tag: cEXIFTT, name: "SceneCaptureType", id: 0xA406},
	{tag: cEXIFTT, name: "GainControl", id: 0xA407},
	{tag: cEXIFTT, name: "Contrast", id: 0xA408},
	{tag: cEXIFTT, name: "Saturation", id: 0xA409},
	{tag: cEXIFTT, name: "Sharpness", id: 0xA40A},
	{tag: cEXIFTT, name: "DeviceSettingDescription", id: 0xA40B},
	{tag: cEXIFTT, name: "SubjectDistanceRange", id: 0xA40C},
	{tag: cEXIFTT, name: "ImageUniqueID", id: 0xA420},
	{tag: cEXIFTT, name: "CameraOwnerName", id: 0xA430},
	{tag: cEXIFTT, name: "BodySerialNumber", id: 0xA431},
	{tag: cEXIFTT, name: "LensSpecification", id: 0xA432},
	{tag: cEXIFTT, name: "LensMake", id: 0xA433},
	{tag: cEXIFTT, name: "LensModel", id: 0xA434},
	{tag: cEXIFTT, name: "LensSerialNumber", id: 0xA435},

	// GPS tags
	{tag: cGPSTT, name: "GPSVersionID", id: 0x0},
	{tag: cGPSTT, name: "GPSLatitudeRef", id: 0x1},
	{tag: cGPSTT, name: "GPSLatitude", id: 0x2},
	{tag: cGPSTT, name: "GPSLongitudeRef", id: 0x3},
	{tag: cGPSTT, name: "GPSLongitude", id: 0x4},
	{tag: cGPSTT, name: "GPSAltitudeRef", id: 0x5},
	{tag: cGPSTT, name: "GPSAltitude", id: 0x6},
	{tag: cGPSTT, name: "GPSTimestamp", id: 0x7},
	{tag: cGPSTT, name: "GPSSatellites", id: 0x8},
	{tag: cGPSTT, name: "GPSStatus", id: 0x9},
	{tag: cGPSTT, name: "GPSMeasureMode", id: 0xA},
	{tag: cGPSTT, name: "GPSDOP", id: 0xB},
	{tag: cGPSTT, name: "GPSSpeedRef", id: 0xC},
	{tag: cGPSTT, name: "GPSSpeed", id: 0xD},
	{tag: cGPSTT, name: "GPSTrackRef", id: 0xE},
	{tag: cGPSTT, name: "GPSTrack", id: 0xF},
	{tag: cGPSTT, name: "GPSImgDirectionRef", id: 0x10},
	{tag: cGPSTT, name: "GPSImgDirection", id: 0x11},
	{tag: cGPSTT, name: "GPSMapDatum", id: 0x12},
	{tag: cGPSTT, name: "GPSDestLatitudeRef", id: 0x13},
	{tag: cGPSTT, name: "GPSDestLatitude", id: 0x14},
	{tag: cGPSTT, name: "GPSDestLongitudeRef", id: 0x15},
	{tag: cGPSTT, name: "GPSDestLongitude", id: 0x16},
	{tag: cGPSTT, name: "GPSDestBearingRef", id: 0x17},
	{tag: cGPSTT, name: "GPSDestBearing", id: 0x18},
	{tag: cGPSTT, name: "GPSDestDistanceRef", id: 0x19},
	{tag: cGPSTT, name: "GPSDestDistance", id: 0x1A},
	{tag: cGPSTT, name: "GPSProcessingMethod", id: 0x1B},
	{tag: cGPSTT, name: "GPSAreaInformation", id: 0x1C},
	{tag: cGPSTT, name: "GPSDateStamp", id: 0x1D},
	{tag: cGPSTT, name: "GPSDifferential", id: 0x1E},
	{tag: cGPSTT, name: "GPSHPositioningError", id: 0x1F},

	// Microsoft Windows metadata. Non-standard, but ubiquitous
	{tag: cIFD0TT, name: "XPTitle", id: 0x9c9b},
	{tag: cIFD0TT, name: "XPComment", id: 0x9c9c},
	{tag: cIFD0TT, name: "XPAuthor", id: 0x9c9d},
	{tag: cIFD0TT, name: "XPKeywords", id: 0x9c9e},
	{tag: cIFD0TT, name: "XPSubject", id: 0x9c9f},
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
