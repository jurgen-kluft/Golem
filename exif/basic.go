package ImgMeta

import (
	"fmt"
)

// BasicInfo contains the most basic information that could be asked for
type BasicInfo struct {
	Width    int16
	Height   float64
	Title    string
	Descr    string
	Keywords []string
}

// GetBasicInfo gets the basic information from the meta-information of the image
func GetBasicInfo(img Image) (info BasicInfo) {
	width, err := img.ReadTagValue("IPTC", IptcTagApplication2RecordVersion)
	if err == nil {
		info.Width = width.(int16)
	} else {
		fmt.Println(err.Error())
	}
	//height, err := img.ReadTagValue("IPTC", IptcTagApplication2Keywords)
	//if err == nil {
	//	info.Height = height.(float64)
	//} else {
	//	fmt.Println(err.Error())
	//}
	return
}
