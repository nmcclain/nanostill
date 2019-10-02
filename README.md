# nanostill: limited clone of raspistill for Jetson Nano
`nanostill` provides a simple CLI tool to capture images on the Jetson Nano.

## Motivation
* The Jetson Nano is similar to the Raspberry Pi, but outperforms it on machine vision tasks.
   * In our tests, the YOLOv3-tiny model can perform detection at about ~2 FPS on a Raspberry Pi 4 and ~20 FPS on a Jetson Nano.
* The Jetson Nano doesn't have a simple image capture tool like `raspistill` - you have to use a [complex](https://developer.ridgerun.com/wiki/index.php?title=Jetson_Nano/Gstreamer/Example_Pipelines/Decoding) [gstreamer](https://gstreamer.freedesktop.org/) [pipeline](https://developer.ridgerun.com/wiki/index.php?title=Jetson_Nano/Gstreamer/Example_Pipelines/Streaming). `nanostill` just wraps that gstreamer pipeline in a command-line tool.
* Commodity pricing: [Jetson Nano DevKit](https://developer.nvidia.com/embedded/jetson-nano-developer-kit) is ~$99 - [5MP camera module](https://www.amazon.com/dp/B07SN8GYGD) is ~$20.

## Installation

* Download nanostill from the [Releases page](../../releases).
* Build from source
   1. Clone this repository: `git clone https://github.com/nmcclain/nanostill.git`
   1. `cd nanostill`
   1. Install dependencies: `go get golang.org/x/image/bmp github.com/docopt/docopt-go github.com/notedit/gstreamer-go github.com/shethchintan7/yuv github.com/sirupsen/logrus`
   1. Build: `go build`

## Usage
`nanostill` supports a small subset of the `raspistill`'s options, as shown below:

```
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
```

### Differences from `raspistill`

* The `-tl` shortcut flag is not supported.  Use the full `--timelapse` argument instead.
* Although the `--hflip` and `--vflip` flags are not supported, you can use the `--flip-method` flag instead.

### Image `--flip-method` options
```
none (0) – Identity (no rotation)
clockwise (1) – Rotate clockwise 90 degrees
rotate-180 (2) – Rotate 180 degrees
counterclockwise (3) – Rotate counter-clockwise 90 degrees
horizontal-flip (4) – Flip horizontally
vertical-flip (5) – Flip vertically
upper-left-diagonal (6) – Flip across upper left/lower right diagonal
upper-right-diagonal (7) – Flip across upper right/lower left diagonal
automatic (8) – Select flip method based on image-orientation tag
```

## Contributing

**Something bugging you?** Please open an Issue or Pull Request - we're here to help!

**New Feature Ideas?** Please open Pull Request, or consider adding one of the missing `raspistill` options!
 
**All Humans Are Equal In This Project And Will Be Treated With Respect.**
