package main

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/bmp"

	docopt "github.com/docopt/docopt-go"
	"github.com/notedit/gstreamer-go"
	"github.com/shethchintan7/yuv"
	log "github.com/sirupsen/logrus"
)

/*
-q, --quality	: Set jpeg quality <0 to 100>
-roi, --roi	: Set region of interest (x,y,w,d as normalised coordinates 0.0-1.0)
 gst-launch-1.0 -v videotestsrc ! videocrop top=42 left=1 right=4 bottom=0 ! ximagesink
*/

const version = "0.1.0"

var usage = `nanostill: limited clone of raspistill for Jetson Nano

Usage:
  nanostill [options] -o <filename>
  nanostill --help
  nanostill --version

Options:
  -o, --output <filename>     REQUIRED: Output filename (to write to stdout, use '-o -')
  -l, --latest <filename>     Link latest complete image to filename
  -t, --timeout <msec>        Time (in ms) before takes picture and shuts down [default: 5000]
  -e, --encoding <encoding>   Encoding to use for output file (jpg, bmp, gif, png) [default: jpg]
  -w, --width <size>          Set image width [default: 3264]
  -h, --height <size>         Set image height [default: 2464]
  --timelapse <msec>          Timelapse mode. Takes a picture every <msec>. %d == frame number (Try: -o img_%04d.jpg)
  -s, --source <source>       Video source (test, nvarguscamera) [default: nvarguscamera]
  --capture-timeout <msec>    Camera image capture timeout (in ms) [default: 1000]
  --flip-method <method_id>   Image flip method (0-8) [default: 0]
  -d, --debug                 Enable debugging messages
  --help                      Show this screen
  --version                   Show version
`

func main() {
	cfg, err := getConfig()
	if err != nil {
		log.Fatalf("Configure error: %v", err)
	}
	frame := 0
	if cfg.timelapse == 0 {
		if err := captureImage(cfg, frame); err != nil {
			log.Fatalf("Capture error: %v", err)
		}
	} else {
		gstOut, err := startGst(cfg)
		if err != nil {
			log.Fatalf("Start GST pipeline error: %s", err)
		}
		for {
			log.Debugf("Capturing...")
			capTimeout := time.NewTicker(cfg.captureTimeout)
			defer capTimeout.Stop()
			select {
			case <-capTimeout.C:
				log.Printf("Capture timeout: exceeded %v", cfg.captureTimeout)
			case buffer := <-gstOut:
				if err := writeImage(cfg, frame, buffer); err != nil {
					log.Errorf("Write image error: %s", err)
				}
			}
			frame++
			time.Sleep(time.Duration(cfg.timelapse) * time.Millisecond)
		}
	}
}

type NSConfig struct {
	output         string
	latest         string
	encoding       string
	gst            string
	source         string
	timeout        time.Duration
	captureTimeout time.Duration
	debug          bool
	timelapse      int
	width          int
	height         int
	flipMethod     int
}

func getConfig() (NSConfig, error) {
	c := NSConfig{}
	args, err := docopt.Parse(usage, nil, true, version, false)
	if err != nil {
		return c, fmt.Errorf("Error parsing args: %s", err.Error())
	}
	c.debug = args["--debug"].(bool)
	if c.debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}
	c.output = args["--output"].(string)
	c.width, err = strconv.Atoi(args["--width"].(string))
	if err != nil {
		return c, fmt.Errorf("Invalid --width, expected # pixels: %s", err.Error())
	}
	c.height, err = strconv.Atoi(args["--height"].(string))
	if err != nil {
		return c, fmt.Errorf("Invalid --height, expected # pixels: %s", err.Error())
	}
	flipUrl := "https://gstreamer.freedesktop.org/documentation/videofilter/videoflip.html?gi-language=python"
	c.flipMethod, err = strconv.Atoi(args["--flip-method"].(string))
	if err != nil {
		return c, fmt.Errorf("Invalid --flip-method, expected # 0-8: %s\n\tSee: %s", err.Error(), flipUrl)
	}
	if c.flipMethod < 0 || c.flipMethod > 8 {
		return c, fmt.Errorf("Invalid --flip-method, expected # 0-8: %s\n\tSee: %s", err.Error(), flipUrl)
	}

	c.latest, _ = args["--latest"].(string)
	timeout, err := strconv.Atoi(args["--timeout"].(string))
	if err != nil {
		return c, fmt.Errorf("Invalid --timeout, expected # msec: %s", err.Error())
	}
	c.timeout = time.Duration(timeout) * time.Millisecond
	timeout, err = strconv.Atoi(args["--capture-timeout"].(string))
	if err != nil {
		return c, fmt.Errorf("Invalid --capture-timeout, expected # msec: %s", err.Error())
	}
	c.captureTimeout = time.Duration(timeout) * time.Millisecond
	c.encoding = args["--encoding"].(string)
	//Encoding to use for output file
	if c.encoding != "jpg" && c.encoding != "bmp" && c.encoding != "gif" && c.encoding != "png" {
		return c, fmt.Errorf("Invalid --encoding, expected jpg, bmp, gif, or png")
	}
	c.source = args["--source"].(string)
	timelapse, ok := args["--timelapse"].(string)
	if ok {
		c.timelapse, err = strconv.Atoi(timelapse)
		if err != nil {
			return c, fmt.Errorf("Invalid --timelapse, expected # msec: %s", err.Error())
		}
	}
	return c, nil
}

type CamResolution struct {
	Width  int
	Height int
	MaxFps int
}

var ValidCamResolutions = []CamResolution{
	CamResolution{Width: 1280, Height: 720, MaxFps: 60},
	CamResolution{Width: 1920, Height: 1080, MaxFps: 30},
	CamResolution{Width: 3264, Height: 1848, MaxFps: 28},
	CamResolution{Width: 3264, Height: 2464, MaxFps: 21},
}

func buildGstPipeline(cfg NSConfig) (string, error) {
	inputFrameRate := 1
	if cfg.timelapse > 0 {
		inputFrameRate = 1000 / cfg.timelapse
		if inputFrameRate < 1 {
			inputFrameRate = 1
		}
	}
	sourceWidth := 0
	sourceHeight := 0
	for _, res := range ValidCamResolutions {
		if res.Width >= cfg.width && res.Height >= cfg.height {
			sourceWidth = res.Width
			sourceHeight = res.Height
			if inputFrameRate > res.MaxFps {
				return "", fmt.Errorf("Couldn't support requested FPS=%d at %dx%d (max FPS=%d) - consider reducing resolution or increasing --timelapse <msec>", inputFrameRate, cfg.width, cfg.height, res.MaxFps)
			}
			break
		}
	}
	if sourceWidth == 0 || sourceHeight == 0 {
		return "", fmt.Errorf("Couldn't find input resolution supporting requested %dx%d", cfg.width, cfg.height)
	}
	log.Debugf("Using input FPS=%d, resolution=%dx%d", inputFrameRate, sourceWidth, sourceHeight)

	gst := ""
	switch cfg.source {
	case "test":
		gst = fmt.Sprintf("videotestsrc ! video/x-raw,format=(string)NV12,framerate=(fraction)%d/1,width=(int)%d,height=(int)%d ! appsink name=sink", inputFrameRate, cfg.width, cfg.height)
	case "nvarguscamera":
		gst = fmt.Sprintf("nvarguscamerasrc ! video/x-raw(memory:NVMM), width=(int)%d, height=(int)%d, format=(string)NV12, framerate=(fraction)%d/1 ! nvvidconv flip-method=%d ! video/x-raw,width=%d,height=%d,format=NV12 ! appsink name=sink", sourceWidth, sourceHeight, inputFrameRate, cfg.flipMethod, cfg.width, cfg.height)
	default:
		return gst, fmt.Errorf("Invalid --source, expected test or nvarguscamera")
	}
	return gst, nil
}

func startGst(cfg NSConfig) (<-chan []byte, error) {
	gst, err := buildGstPipeline(cfg)
	if err != nil {
		return nil, err
	}
	log.Debugf("GStreamer pipeline: gst-launch-1.0 %s", gst)
	pipeline, err := gstreamer.New(gst)
	if err != nil {
		return nil, err
	}

	appsink := pipeline.FindElement("sink")
	pipeline.Start()
	out := appsink.Poll()
	return out, nil
}

func captureImage(cfg NSConfig, frame int) error {
	gstOut, err := startGst(cfg)
	if err != nil {
		return err
	}
	if cfg.timeout > 0 {
		log.Debugf("Sleeping %+v...", cfg.timeout)
		time.Sleep(cfg.timeout)
	}
	log.Debugf("Capturing...")

	capTimeout := time.NewTicker(cfg.captureTimeout)
	defer capTimeout.Stop()
	select {
	case <-capTimeout.C:
		return fmt.Errorf("Capture timeout: exceeded %v", cfg.captureTimeout)
	case buffer := <-gstOut:
		if err := writeImage(cfg, frame, buffer); err != nil {
			return err
		}
	}
	return nil
}

func writeImage(cfg NSConfig, frame int, buffer []byte) error {
	fname := ""
	var w io.WriteCloser
	if cfg.output == "-" { // stdout
		w = os.Stdout
	} else {
		var err error
		fname = getFileName(cfg.output, frame)
		w, err = os.Create(fname)
		if err != nil {
			return err
		}
	}
	newimage := yuv.NewYUV(image.Rect(0, 0, cfg.width, cfg.height), yuv.NV12)
	newimage.Y = buffer[0 : cfg.width*cfg.height]
	newimage.U = buffer[cfg.width*cfg.height:]
	newimage.V = buffer[cfg.width*cfg.height+1:]

	switch cfg.encoding {
	case "jpg":
		jpegOptions := jpeg.Options{Quality: 75} // TODO
		jpeg.Encode(w, newimage, &jpegOptions)
		w.Close()
	case "gif":
		gif.Encode(w, newimage, nil)
		w.Close()
	case "bmp":
		bmp.Encode(w, newimage)
		w.Close()
	case "png":
		png.Encode(w, newimage)
		w.Close()
	default:
		w.Close()
		return fmt.Errorf("Unsupported encoding %s", cfg.encoding)
	}
	if cfg.output != "-" { // stdout
		log.Debugf("Wrote %s: %s", cfg.encoding, fname)
		if len(cfg.latest) > 0 {
			_ = os.Remove(cfg.latest)
			if err := os.Symlink(fname, cfg.latest); err != nil {
				return err
			}
			log.Debugf("Linked %s -> %s", cfg.latest, fname)
		}
	}

	return nil
}

var getFileNameRe = regexp.MustCompile(`(%\d{0,99}d)`)

func getFileName(fname string, frame int) string {
	m := getFileNameRe.FindStringSubmatch(fname)
	if len(m) < 2 {
		return fname
	}
	return strings.Replace(fname, m[1], fmt.Sprintf(m[1], frame), 1)
}
