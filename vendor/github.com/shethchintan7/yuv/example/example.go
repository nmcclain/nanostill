package main

import (
	"image"
	"image/jpeg"
	"io/ioutil"
	"os"

	"github.com/shethchintan7/yuv"
)

func main() {
	input, _ := ioutil.ReadFile("ImageData.RAW")
	w, _ := os.Create("output.jpeg")

	newimage := yuv.NewYUV(image.Rect(0, 0, 3840, 2160), yuv.NV12)
	newimage.Y = input[0 : 3840*2160]
	newimage.U = input[3840*2160:]
	newimage.V = input[3840*2160+1:]

	jpeg.Encode(w, newimage, nil)
}
