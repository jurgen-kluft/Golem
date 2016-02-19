package ImgMeta

import (
	"fmt"
)

// BasicInfo contains the most basic information that could be asked for
type BasicInfo struct {
	Width    float64
	Height   float64
	Title    string
	Descr    string
	Keywords []string
}

// GetBasicInfo gets the basic information from the meta-information of the image
func GetBasicInfo(img Image) (info BasicInfo) {
	width, err := img.ReadTagValue("EXIF", ExifTagXResolution)
	if err == nil {
		info.Width = width.(float64)
	} else {
		fmt.Println(err.Error())
	}
	height, err := img.ReadTagValue("EXIF", ExifTagYResolution)
	if err == nil {
		info.Height = height.(float64)
	} else {
		fmt.Println(err.Error())
	}
	return
}
