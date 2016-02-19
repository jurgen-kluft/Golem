package ImgMeta

/*
Structure of a JFIF APP0 segment

APP0 segments are used in the old JFIF standard to store information about the picture dimensions and an optional thumbnail.
The format of a JFIF APP0 segment is as follows (note that the size of thumbnail data is 3n, where n = Xthumbnail * Ythumbnail,
and it is present only if n is not zero; only the first 8 records are mandatory):

    [Record name]    [size]   [description]
    ---------------------------------------
    Identifier       5 bytes  ("JFIF\000" = 0x4a46494600)
    MajorVersion     1 byte   major version (e.g. 0x01)
    MinorVersion     1 byte   minor version (e.g. 0x01 or 0x02)
    Units            1 byte   units (0: densities give aspect ratio
                                     1: density values are dots per inch
                                     2: density values are dots per cm)
    Xdensity         2 bytes  horizontal pixel density
    Ydensity         2 bytes  vertical pixel density
    Xthumbnail       1 byte   thumbnail horizontal pixel count
    Ythumbnail       1 byte   thumbnail vertical pixel count
    ThumbnailData   3n bytes  thumbnail image

There is also an extended JFIF (only possible for JFIF versions 1.02 and above). In this case the identifier is not JFIF but JFXX.
This extension allows for the inclusion of differently encoded thumbnails. The syntax in this case is modified as follows:

    [Record name]    [size]   [description]
    ---------------------------------------
    Identifier       5 bytes  ("JFXX\000" = 0x4a46585800)
    ExtensionCode    1 byte   (0x10 Thumbnail coded using JPEG
                               0x11 Thumbnail using 1 byte/pixel
                               0x13 Thumbnail using 3 bytes/pixel)

Then, depending on the extension code, there are other records to define the thumbnail. If the thumbnail is coded using a JPEG stream,
a binary JPEG stream immediately follows the extension code (the byte count of this file is included in the byte count of the APP0 Segment).
This stream conforms to the syntax for a JPEG file (SOI .... SOF ... EOI); however, no 'JFIF' or 'JFXX' marker Segments should be present:

    [Record name]    [size]   [description]
    ---------------------------------------
    JPEGThumbnail  ... bytes  a variable length JPEG picture

If the thumbnail is stored using one byte per pixel, after the extension code one should find a palette and an indexed RGB.
The records are as follows (remember that n = Xthumbnail * Ythumbnail):

    [Record name]    [size]   [description]
    ---------------------------------------
    Xthumbnail       1 byte    thumbnail horizontal pixel count
    YThumbnail       1 byte    thumbnail vertical pixel count
    ColorPalette   768 bytes   24-bit RGB values for the colour palette
                               (defining the colours represented by each
                                value of an 8-bit binary encoding)
    1ByteThumbnail   n bytes   8-bit indexed values for the thumbnail

If the thumbnail is stored using three bytes per pixel, there is no colour palette, so the previous fields simplify into:

    [Record name]    [size]   [description]
    ---------------------------------------
    Xthumbnail       1 byte    thumbnail horizontal pixel count
    YThumbnail       1 byte    thumbnail vertical pixel count
    3BytesThumbnail 3n bytes 24-bit RGB values for the thumbnail

*/
