package main

import (
	"fmt"
	"github.com/jurgen-kluft/golem/exif"
	"os"
)

func main() {
	fhnd, err := os.Open("test.jpg")
	if err != nil {
		return
	}

	image, err := ImgMeta.ReadJpeg(fhnd)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	basicInfo := ImgMeta.GetBasicInfo(image)
	fmt.Printf("Image: width:%f, height:%f\n", basicInfo.Width, basicInfo.Height)
}
