package EXIF

import ()

// BasicInfo contains the most basic information that could be asked for
type BasicInfo struct {
	Width    int
	Height   int
	Title    string
	Descr    string
	Keywords []string
}

// GetBasicInfo gets the basic information from the meta-information of the image
func GetBasicInfo(img Image) (info BasicInfo) {

	return
}
