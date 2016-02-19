package ImgMeta

import (
	"encoding/binary"
	"fmt"
)

/*
Structure of a Photoshop-style APP13 segment

The Adobe's Photoshop program, a de-facto standard for image manipulation, uses the APP13 segment for storing non-graphic
information, such as layers, paths, IPTC data and more. The unit for this kind of information is called a "resource data block"
(because they hold data that was stored in the Macintosh's resource fork in early versions of Photoshop).
The content of an APP13 segment is formed by an identifier string (usually "Photoshop 3.0\000", but also 'Adobe_Photoshop2.5:',
used by earlier versions, is accepted; in this case some additional undocumented bytes are read (resolution info?) and saved in
a root 'Resolution' record) followed by a sequence of resource data blocks; a resource block has the following structure:

    [Record name]    [size]   [description]
    ---------------------------------------
    (Type)           4 bytes  Photoshop uses '8BIM' from ver 4.0 on
    (ID)             2 bytes  a unique identifier, e.g., "\004\004" for IPTC
    (Name)             ...    a Pascal string (padded to make size even)
    (Size)           4 bytes  actual size of resource data
    (Data)             ...    resource data, padded to make size even

(a Pascal string is made up of a single byte, giving the string length, followed by the string itself, padded to make size
even including the length byte; since the string length is explicit, there is no need of a terminating null character).
The signature (type) is usually '8BIM', but Photoshop used '8BPS' up to version 3.0, and some rogue program (Adobe PhotoDeluxe?)
is using 'PHUT' ("PHotoshop User Tags" ?) for path information (ID=7d0-bb7). Valid Image Resource IDs are listed in the
Photoshop-style tags' list section. In general a resource block contains only a few bytes, but there is an important block,
the IPTC block, which can be quite large; the structure of this block is analysed in more detail in the IPTC data block section.

*/

/*
Structure of an IPTC data block

An IPTC/NAA resource data block of a Photoshop-style APP13 segment embeds an IPTC stream conforming to the standard defined by
the International Press and Telecommunications Council (IPTC) and the Newspaper Association of America (NAA) for exchanging
interoperability information related to various news objects. The data part of a resource block, an IPTC stream, is simply a
sequence of units called datasets; no preamble nor count is present. Each dataset consists of a unique tag header and a data
field (the list of valid tags [dataset numbers] can be found in section about IPTC data). A standard tag header is used when
the data field size is less than 32768 bytes; otherwise, an extended tag header is used. The datasets do not need to show up
in numerical order according to their tag. The structure of a dataset is:

    [Record name]    [size]   [description]
    ---------------------------------------
    (Tag marker)     1 byte   this must be 0x1c
    (Record number)  1 byte   always 2 for 2:xx datasets
    (Dataset number) 1 byte   this is what we call a "tag"
    (Size specifier) 2 bytes  data length (< 32768 bytes) or length of ...
    (Size specifier)  ...     data length (> 32767 bytes only)
    (Data)            ...     (its length is specified before)

So, standard datasets have a 5 bytes tag header; the last two bytes in the header contain the data field length, the most
significant bit being always 0. For extended datasets instead, these two bytes contain the length of the (following) data
field length, the most significant bit being always 1. The value of the most significant bit thus distinguishes "standard"
from "extended"; in digital photographies, I assume that the datasets which are actually used (a subset of the standard) are
always standard; therefore, we likely do not have the IPTC block spanning more than one APP13 segment. The record types
defined by the IPTC-NAA standard are the following (but the "pseudo"-standard by Adobe for APP13 IPTC data is restricted to
the first application record, 2:xx, and sometimes to the envelope record, 1:xx, I believe, because everything else can be
accomodated more simply by other JPEG Segments):

    [Record name]                [dataset record number]
    ----------------------------------------------------
    Object Envelop Record                 1:xx
    Application Records:             2:xx through 6:xx
    Pre-ObjectData Descriptor Record:     7:xx
    ObjectData Record:                    8:xx
    Post-ObjectData Descriptor Record:    9:xx

*/

type tIPTCAPP struct {
	name   string
	offset uint64           // Offset of this APP in the file
	endian binary.ByteOrder // Byte-Order
	block  []byte           // full APP block
}

func (t tIPTCAPP) Name() string {
	return t.name
}
func (t tIPTCAPP) Marker() uint16 {
	return t.endian.Uint16(t.block)
}
func (t tIPTCAPP) Length() uint16 {
	return t.endian.Uint16(t.block[2:])
}
func (t tIPTCAPP) ID(cid []byte) (id []byte) {
	id = t.block[4 : 4+len(cid)]
	return
}
func (t tIPTCAPP) HasID(cid []byte) bool {
	id := t.block[4 : 4+len(cid)]
	for i, b := range id {
		if b != cid[i] {
			return false
		}
	}
	return true
}

type tIPTCHeader struct {
	block  []byte
	endian binary.ByteOrder // Byte-Order
}

type tIPTCRecordReader struct {
	block  []byte
	endian binary.ByteOrder // Byte-Order
	cursor uint32
}

func (t tIPTCRecordReader) Tag() uint16 {
	return t.endian.Uint16(t.block[t.cursor:])
}
func (t tIPTCRecordReader) Size() uint16 {
	return t.endian.Uint16(t.block[t.cursor+1:])
}
func (t tIPTCRecordReader) Next() bool {
	t.cursor += uint32(t.Size())
	return t.cursor < uint32(len(t.block))
}

func (t tIPTCHeader) Has8BIM() bool {
	return t.block[0] == '8' && t.block[1] == 'B' && (t.block[2] == 'I' || t.block[2] == 'P') && (t.block[3] == 'M' || t.block[3] == 'S')
}
func (t tIPTCHeader) HasIPTCID() bool {
	return t.block[4] == 4 && t.block[5] == 4
}
func (t tIPTCHeader) NameLen() uint32 {
	return uint32(t.block[6])
}
func (t tIPTCHeader) Size() uint32 {
	offset := 6 + t.NameLen()
	return t.endian.Uint32(t.block[offset:])
}

func (t tIPTCAPP) ReadValue(tagID2Find uint32) (interface{}, error) {
	fmt.Printf("IPTC, marker:")

	// @WORK IN PROGRESS

	for i := 0; i < 10; i++ {
		b := t.block[14+4+i]
		fmt.Printf("%X ", b)
	}
	fmt.Printf("\n")
	return int(0), &exifError{"Reading IPTC tag value failed"}
}

const (
	IptcTagGroupEnvelope    = 0x1000
	IptcTagGroupApplication = 0x2000
)

const (
	IptcTagEnvelopeModelVersion              = IptcTagGroupEnvelope | 0x0000
	IptcTagEnvelopeDestination               = IptcTagGroupEnvelope | 0x0005
	IptcTagEnvelopeFileFormat                = IptcTagGroupEnvelope | 0x0014
	IptcTagEnvelopeFileVersion               = IptcTagGroupEnvelope | 0x0016
	IptcTagEnvelopeServiceID                 = IptcTagGroupEnvelope | 0x001e
	IptcTagEnvelopeEnvelopeNumber            = IptcTagGroupEnvelope | 0x0028
	IptcTagEnvelopeProductID                 = IptcTagGroupEnvelope | 0x0032
	IptcTagEnvelopeEnvelopePriority          = IptcTagGroupEnvelope | 0x003c
	IptcTagEnvelopeDateSent                  = IptcTagGroupEnvelope | 0x0046
	IptcTagEnvelopeTimeSent                  = IptcTagGroupEnvelope | 0x0050
	IptcTagEnvelopeCharacterSet              = IptcTagGroupEnvelope | 0x005a
	IptcTagEnvelopeUNO                       = IptcTagGroupEnvelope | 0x0064
	IptcTagEnvelopeARMId                     = IptcTagGroupEnvelope | 0x0078
	IptcTagEnvelopeARMVersion                = IptcTagGroupEnvelope | 0x007a
	IptcTagApplication2RecordVersion         = IptcTagGroupApplication | 0x0000
	IptcTagApplication2ObjectType            = IptcTagGroupApplication | 0x0003
	IptcTagApplication2ObjectAttribute       = IptcTagGroupApplication | 0x0004
	IptcTagApplication2ObjectName            = IptcTagGroupApplication | 0x0005
	IptcTagApplication2EditStatus            = IptcTagGroupApplication | 0x0007
	IptcTagApplication2EditorialUpdate       = IptcTagGroupApplication | 0x0008
	IptcTagApplication2Urgency               = IptcTagGroupApplication | 0x000a
	IptcTagApplication2Subject               = IptcTagGroupApplication | 0x000c
	IptcTagApplication2Category              = IptcTagGroupApplication | 0x000f
	IptcTagApplication2SuppCategory          = IptcTagGroupApplication | 0x0014
	IptcTagApplication2FixtureID             = IptcTagGroupApplication | 0x0016
	IptcTagApplication2Keywords              = IptcTagGroupApplication | 0x0019
	IptcTagApplication2LocationCode          = IptcTagGroupApplication | 0x001a
	IptcTagApplication2LocationName          = IptcTagGroupApplication | 0x001b
	IptcTagApplication2ReleaseDate           = IptcTagGroupApplication | 0x001e
	IptcTagApplication2ReleaseTime           = IptcTagGroupApplication | 0x0023
	IptcTagApplication2ExpirationDate        = IptcTagGroupApplication | 0x0025
	IptcTagApplication2ExpirationTime        = IptcTagGroupApplication | 0x0026
	IptcTagApplication2SpecialInstructions   = IptcTagGroupApplication | 0x0028
	IptcTagApplication2ActionAdvised         = IptcTagGroupApplication | 0x002a
	IptcTagApplication2ReferenceService      = IptcTagGroupApplication | 0x002d
	IptcTagApplication2ReferenceDate         = IptcTagGroupApplication | 0x002f
	IptcTagApplication2ReferenceNumber       = IptcTagGroupApplication | 0x0032
	IptcTagApplication2DateCreated           = IptcTagGroupApplication | 0x0037
	IptcTagApplication2TimeCreated           = IptcTagGroupApplication | 0x003c
	IptcTagApplication2DigitizationDate      = IptcTagGroupApplication | 0x003e
	IptcTagApplication2DigitizationTime      = IptcTagGroupApplication | 0x003f
	IptcTagApplication2Program               = IptcTagGroupApplication | 0x0041
	IptcTagApplication2ProgramVersion        = IptcTagGroupApplication | 0x0046
	IptcTagApplication2ObjectCycle           = IptcTagGroupApplication | 0x004b
	IptcTagApplication2Byline                = IptcTagGroupApplication | 0x0050
	IptcTagApplication2BylineTitle           = IptcTagGroupApplication | 0x0055
	IptcTagApplication2City                  = IptcTagGroupApplication | 0x005a
	IptcTagApplication2SubLocation           = IptcTagGroupApplication | 0x005c
	IptcTagApplication2ProvinceState         = IptcTagGroupApplication | 0x005f
	IptcTagApplication2CountryCode           = IptcTagGroupApplication | 0x0064
	IptcTagApplication2CountryName           = IptcTagGroupApplication | 0x0065
	IptcTagApplication2TransmissionReference = IptcTagGroupApplication | 0x0067
	IptcTagApplication2Headline              = IptcTagGroupApplication | 0x0069
	IptcTagApplication2Credit                = IptcTagGroupApplication | 0x006e
	IptcTagApplication2Source                = IptcTagGroupApplication | 0x0073
	IptcTagApplication2Copyright             = IptcTagGroupApplication | 0x0074
	IptcTagApplication2Contact               = IptcTagGroupApplication | 0x0076
	IptcTagApplication2Caption               = IptcTagGroupApplication | 0x0078
	IptcTagApplication2Writer                = IptcTagGroupApplication | 0x007a
	IptcTagApplication2RasterizedCaption     = IptcTagGroupApplication | 0x007d
	IptcTagApplication2ImageType             = IptcTagGroupApplication | 0x0082
	IptcTagApplication2ImageOrientation      = IptcTagGroupApplication | 0x0083
	IptcTagApplication2Language              = IptcTagGroupApplication | 0x0087
	IptcTagApplication2AudioType             = IptcTagGroupApplication | 0x0096
	IptcTagApplication2AudioRate             = IptcTagGroupApplication | 0x0097
	IptcTagApplication2AudioResolution       = IptcTagGroupApplication | 0x0098
	IptcTagApplication2AudioDuration         = IptcTagGroupApplication | 0x0099
	IptcTagApplication2AudioOutcue           = IptcTagGroupApplication | 0x009a
	IptcTagApplication2PreviewFormat         = IptcTagGroupApplication | 0x00c8
	IptcTagApplication2PreviewVersion        = IptcTagGroupApplication | 0x00c9
	IptcTagApplication2Preview               = IptcTagGroupApplication | 0x00ca
)

type tIPTCField struct {
	tagTypeID      uint16
	fieldTypeID    uint16
	isMandatory    bool
	isRepeatable   bool
	minSizeInBytes int
	maxSizeInBytes int
	description    string
}

const (
	IptcFieldTypeShort     uint16 = iota
	IptcFieldTypeString    uint16 = iota
	IptcFieldTypeDate      uint16 = iota
	IptcFieldTypeTime      uint16 = iota
	IptcFieldTypeUndefined uint16 = iota
)

const (
	Yes bool = true
	No  bool = false
)

var ArrayOfIPTCFields = map[uint16]tIPTCField{
	IptcTagEnvelopeModelVersion:              {IptcTagEnvelopeModelVersion, IptcFieldTypeShort, Yes, No, 2, 2, "A binary number identifying the version of the Information Interchange Model"},
	IptcTagEnvelopeDestination:               {IptcTagEnvelopeDestination, IptcFieldTypeString, No, Yes, 0, 1024, "This DataSet is to accommodate some providers who require routing information above the appropriate OSI layers."},
	IptcTagEnvelopeFileFormat:                {IptcTagEnvelopeFileFormat, IptcFieldTypeShort, Yes, No, 2, 2, "A binary number representing the file format. The file format must be registered with IPTC or NAA with a unique number assigned to it. The information is used to route the data to the appropriate system and to allow the receiving system to perform the appropriate actions there to."},
	IptcTagEnvelopeFileVersion:               {IptcTagEnvelopeFileVersion, IptcFieldTypeShort, Yes, No, 2, 2, "A binary number representing the particular version of the File Format specified by <FileFormat> tag."},
	IptcTagEnvelopeServiceID:                 {IptcTagEnvelopeServiceID, IptcFieldTypeString, Yes, No, 0, 10, "Identifies the provider and product"},
	IptcTagEnvelopeEnvelopeNumber:            {IptcTagEnvelopeEnvelopeNumber, IptcFieldTypeString, Yes, No, 8, 8, "The characters form a number that will be unique for the date specified in <DateSent> tag and for the Service Identifier specified by <ServiceIdentifier> tag. If identical envelope numbers appear with the same date and with the same Service Identifier"},
	IptcTagEnvelopeProductID:                 {IptcTagEnvelopeProductID, IptcFieldTypeString, No, Yes, 0, 32, "Allows a provider to identify subsets of its overall service. Used to provide receiving organisation data on which to select"},
	IptcTagEnvelopeEnvelopePriority:          {IptcTagEnvelopeEnvelopePriority, IptcFieldTypeString, No, No, 1, 1, "Specifies the envelope handling priority and not the editorial urgency (see <Urgency> tag). '1' indicates the most urgent"},
	IptcTagEnvelopeDateSent:                  {IptcTagEnvelopeDateSent, IptcFieldTypeDate, Yes, No, 8, 8, "Uses the format CCYYMMDD (century"},
	IptcTagEnvelopeTimeSent:                  {IptcTagEnvelopeTimeSent, IptcFieldTypeTime, No, No, 1, 11, "Uses the format HHMMSS:HHMM where HHMMSS refers to local hour"},
	IptcTagEnvelopeCharacterSet:              {IptcTagEnvelopeCharacterSet, IptcFieldTypeString, No, No, 0, 32, "This tag consisting of one or more control functions used for the announcement"},
	IptcTagEnvelopeUNO:                       {IptcTagEnvelopeUNO, IptcFieldTypeString, No, No, 4, 80, "This tag provide a globally unique identification for objects as specified in the IIM"},
	IptcTagEnvelopeARMId:                     {IptcTagEnvelopeARMId, IptcFieldTypeShort, No, No, 2, 2, "The DataSet identifies the Abstract Relationship Method identifier (ARM) which is described in a document registered by the originator of the ARM with the IPTC and NAA organizations."},
	IptcTagEnvelopeARMVersion:                {IptcTagEnvelopeARMVersion, IptcFieldTypeShort, No, No, 2, 2, "This tag consisting of a binary number representing the particular version of the ARM specified by tag <ARMId>."},
	IptcTagApplication2RecordVersion:         {IptcTagApplication2RecordVersion, IptcFieldTypeShort, Yes, No, 2, 2, "A binary number identifying the version of the Information Interchange Model"},
	IptcTagApplication2ObjectType:            {IptcTagApplication2ObjectType, IptcFieldTypeString, No, No, 3, 67, "The Object Type is used to distinguish between different types of objects within the IIM. The first part is a number representing a language independent international reference to an Object Type followed by a colon separator. The second part"},
	IptcTagApplication2ObjectAttribute:       {IptcTagApplication2ObjectAttribute, IptcFieldTypeString, No, Yes, 4, 68, "The Object Attribute defines the nature of the object independent of the Subject. The first part is a number representing a language independent international reference to an Object Attribute followed by a colon separator. The second part"},
	IptcTagApplication2ObjectName:            {IptcTagApplication2ObjectName, IptcFieldTypeString, No, No, 0, 64, "Used as a shorthand reference for the object. Changes to exist-ing data"},
	IptcTagApplication2EditStatus:            {IptcTagApplication2EditStatus, IptcFieldTypeString, No, No, 0, 64, "Status of the object data"},
	IptcTagApplication2EditorialUpdate:       {IptcTagApplication2EditorialUpdate, IptcFieldTypeString, No, No, 2, 2, "Indicates the type of update that this object provides to a previous object. The link to the previous object is made using the tags <ARMIdentifier> and <ARMVersion>"},
	IptcTagApplication2Urgency:               {IptcTagApplication2Urgency, IptcFieldTypeString, No, No, 1, 1, "Specifies the editorial urgency of content and not necessarily the envelope handling priority (see tag <EnvelopePriority>). The '1' is most urgent"},
	IptcTagApplication2Subject:               {IptcTagApplication2Subject, IptcFieldTypeString, No, Yes, 3, 236, "The Subject Reference is a structured definition of the subject matter."},
	IptcTagApplication2Category:              {IptcTagApplication2Category, IptcFieldTypeString, No, No, 0, 3, "Identifies the subject of the object data in the opinion of the provider. A list of categories will be maintained by a regional registry"},
	IptcTagApplication2SuppCategory:          {IptcTagApplication2SuppCategory, IptcFieldTypeString, No, Yes, 0, 32, "Supplemental categories further refine the subject of an object data. A supplemental category may include any of the recognised categories as used in tag <Category>. Otherwise"},
	IptcTagApplication2FixtureID:             {IptcTagApplication2FixtureID, IptcFieldTypeString, No, No, 0, 32, "Identifies object data that recurs often and predictably. Enables users to immediately find or recall such an object."},
	IptcTagApplication2Keywords:              {IptcTagApplication2Keywords, IptcFieldTypeString, No, Yes, 0, 64, "Used to indicate specific information retrieval words. It is expected that a provider of various types of data that are related in subject matter uses the same keyword"},
	IptcTagApplication2LocationCode:          {IptcTagApplication2LocationCode, IptcFieldTypeString, No, Yes, 3, 3, "Indicates the code of a country/geographical location referenced by the content of the object. Where ISO has established an appropriate country code under ISO 3166"},
	IptcTagApplication2LocationName:          {IptcTagApplication2LocationName, IptcFieldTypeString, No, Yes, 0, 64, "Provides a full"},
	IptcTagApplication2ReleaseDate:           {IptcTagApplication2ReleaseDate, IptcFieldTypeDate, No, No, 8, 8, "Designates in the form CCYYMMDD the earliest date the provider intends the object to be used. Follows ISO 8601 standard."},
	IptcTagApplication2ReleaseTime:           {IptcTagApplication2ReleaseTime, IptcFieldTypeTime, No, No, 1, 11, "Designates in the form HHMMSS:HHMM the earliest time the provider intends the object to be used. Follows ISO 8601 standard."},
	IptcTagApplication2ExpirationDate:        {IptcTagApplication2ExpirationDate, IptcFieldTypeDate, No, No, 8, 8, "Designates in the form CCYYMMDD the latest date the provider or owner intends the object data to be used. Follows ISO 8601 standard."},
	IptcTagApplication2ExpirationTime:        {IptcTagApplication2ExpirationTime, IptcFieldTypeTime, No, No, 1, 11, "Designates in the form HHMMSS:HHMM the latest time the provider or owner intends the object data to be used. Follows ISO 8601 standard."},
	IptcTagApplication2SpecialInstructions:   {IptcTagApplication2SpecialInstructions, IptcFieldTypeString, No, No, 0, 256, "Other editorial instructions concerning the use of the object data"},
	IptcTagApplication2ActionAdvised:         {IptcTagApplication2ActionAdvised, IptcFieldTypeString, No, No, 2, 2, "Indicates the type of action that this object provides to a previous object. The link to the previous object is made using tags <ARMIdentifier> and <ARMVersion>"},
	IptcTagApplication2ReferenceService:      {IptcTagApplication2ReferenceService, IptcFieldTypeString, No, Yes, 0, 10, "Identifies the Service Identifier of a prior envelope to which the current object refers."},
	IptcTagApplication2ReferenceDate:         {IptcTagApplication2ReferenceDate, IptcFieldTypeDate, No, Yes, 8, 8, "Identifies the date of a prior envelope to which the current object refers."},
	IptcTagApplication2ReferenceNumber:       {IptcTagApplication2ReferenceNumber, IptcFieldTypeString, No, Yes, 8, 8, "Identifies the Envelope Number of a prior envelope to which the current object refers."},
	IptcTagApplication2DateCreated:           {IptcTagApplication2DateCreated, IptcFieldTypeDate, No, No, 8, 8, "Represented in the form CCYYMMDD to designate the date the intellectual content of the object data was created rather than the date of the creation of the physical representation. Follows ISO 8601 standard."},
	IptcTagApplication2TimeCreated:           {IptcTagApplication2TimeCreated, IptcFieldTypeTime, No, No, 1, 11, "Represented in the form HHMMSS:HHMM to designate the time the intellectual content of the object data current source material was created rather than the creation of the physical representation. Follows ISO 8601 standard."},
	IptcTagApplication2DigitizationDate:      {IptcTagApplication2DigitizationDate, IptcFieldTypeDate, No, No, 8, 8, "Represented in the form CCYYMMDD to designate the date the digital representation of the object data was created. Follows ISO 8601 standard."},
	IptcTagApplication2DigitizationTime:      {IptcTagApplication2DigitizationTime, IptcFieldTypeTime, No, No, 1, 11, "Represented in the form HHMMSS:HHMM to designate the time the digital representation of the object data was created. Follows ISO 8601 standard."},
	IptcTagApplication2Program:               {IptcTagApplication2Program, IptcFieldTypeString, No, No, 0, 32, "Identifies the type of program used to originate the object data."},
	IptcTagApplication2ProgramVersion:        {IptcTagApplication2ProgramVersion, IptcFieldTypeString, No, No, 0, 10, "Used to identify the version of the program mentioned in tag <Program>."},
	IptcTagApplication2ObjectCycle:           {IptcTagApplication2ObjectCycle, IptcFieldTypeString, No, No, 1, 1, "Used to identify the editorial cycle of object data."},
	IptcTagApplication2Byline:                {IptcTagApplication2Byline, IptcFieldTypeString, No, Yes, 0, 32, "Contains name of the creator of the object data"},
	IptcTagApplication2BylineTitle:           {IptcTagApplication2BylineTitle, IptcFieldTypeString, No, Yes, 0, 32, "A by-line title is the title of the creator or creators of an object data. Where used"},
	IptcTagApplication2City:                  {IptcTagApplication2City, IptcFieldTypeString, No, No, 0, 32, "Identifies city of object data origin according to guidelines established by the provider."},
	IptcTagApplication2SubLocation:           {IptcTagApplication2SubLocation, IptcFieldTypeString, No, No, 0, 32, "Identifies the location within a city from which the object data originates"},
	IptcTagApplication2ProvinceState:         {IptcTagApplication2ProvinceState, IptcFieldTypeString, No, No, 0, 32, "Identifies Province/State of origin according to guidelines established by the provider."},
	IptcTagApplication2CountryCode:           {IptcTagApplication2CountryCode, IptcFieldTypeString, No, No, 3, 3, "Indicates the code of the country/primary location where the intellectual property of the object data was created"},
	IptcTagApplication2CountryName:           {IptcTagApplication2CountryName, IptcFieldTypeString, No, No, 0, 64, "Provides full"},
	IptcTagApplication2TransmissionReference: {IptcTagApplication2TransmissionReference, IptcFieldTypeString, No, No, 0, 32, "A code representing the location of original transmission according to practices of the provider."},
	IptcTagApplication2Headline:              {IptcTagApplication2Headline, IptcFieldTypeString, No, No, 0, 256, "A publishable entry providing a synopsis of the contents of the object data."},
	IptcTagApplication2Credit:                {IptcTagApplication2Credit, IptcFieldTypeString, No, No, 0, 32, "Identifies the provider of the object data"},
	IptcTagApplication2Source:                {IptcTagApplication2Source, IptcFieldTypeString, No, No, 0, 32, "Identifies the original owner of the intellectual content of the object data. This could be an agency"},
	IptcTagApplication2Copyright:             {IptcTagApplication2Copyright, IptcFieldTypeString, No, No, 0, 128, "Contains any necessary copyright notice."},
	IptcTagApplication2Contact:               {IptcTagApplication2Contact, IptcFieldTypeString, No, Yes, 0, 128, "Identifies the person or organisation which can provide further background information on the object data."},
	IptcTagApplication2Caption:               {IptcTagApplication2Caption, IptcFieldTypeString, No, No, 0, 2000, "A textual description of the object data."},
	IptcTagApplication2Writer:                {IptcTagApplication2Writer, IptcFieldTypeString, No, Yes, 0, 32, "Identification of the name of the person involved in the writing"},
	IptcTagApplication2RasterizedCaption:     {IptcTagApplication2RasterizedCaption, IptcFieldTypeUndefined, No, No, 0, 7360, "Contains the rasterized object data description and is used where characters that have not been coded are required for the caption."},
	IptcTagApplication2ImageType:             {IptcTagApplication2ImageType, IptcFieldTypeString, No, No, 2, 2, "Indicates the color components of an image."},
	IptcTagApplication2ImageOrientation:      {IptcTagApplication2ImageOrientation, IptcFieldTypeString, No, No, 1, 1, "Indicates the layout of an image."},
	IptcTagApplication2Language:              {IptcTagApplication2Language, IptcFieldTypeString, No, No, 2, 3, "Describes the major national language of the object"},
	IptcTagApplication2AudioType:             {IptcTagApplication2AudioType, IptcFieldTypeString, No, No, 2, 2, "Indicates the type of an audio content."},
	IptcTagApplication2AudioRate:             {IptcTagApplication2AudioRate, IptcFieldTypeString, No, No, 6, 6, "Indicates the sampling rate in Hertz of an audio content."},
	IptcTagApplication2AudioResolution:       {IptcTagApplication2AudioResolution, IptcFieldTypeString, No, No, 2, 2, "Indicates the sampling resolution of an audio content."},
	IptcTagApplication2AudioDuration:         {IptcTagApplication2AudioDuration, IptcFieldTypeString, No, No, 6, 6, "Indicates the duration of an audio content."},
	IptcTagApplication2AudioOutcue:           {IptcTagApplication2AudioOutcue, IptcFieldTypeString, No, No, 0, 64, "Identifies the content of the end of an audio object data"},
	IptcTagApplication2PreviewFormat:         {IptcTagApplication2PreviewFormat, IptcFieldTypeShort, No, No, 2, 2, "A binary number representing the file format of the object data preview. The file format must be registered with IPTC or NAA organizations with a unique number assigned to it."},
	IptcTagApplication2PreviewVersion:        {IptcTagApplication2PreviewVersion, IptcFieldTypeShort, No, No, 2, 2, "A binary number representing the particular version of the object data preview file format specified in tag <PreviewFormat>."},
	IptcTagApplication2Preview:               {IptcTagApplication2Preview, IptcFieldTypeUndefined, No, No, 0, 256000, "Binary image preview data."},
}
