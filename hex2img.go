package main

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/ajstarks/svgo"
)

const (
	pixelSize = 8
)

func main() {
	decode := flag.Bool("d", false, "Decode PNG/SVG to hex")
	blocksPerRow := flag.Int("b", 0, "Number of blocks per row (0 for single row)")
	useSVG := flag.Bool("v", false, "Use SVG format instead of PNG")
	help := flag.Bool("h", false, "Show help")
	flag.Parse()

	if *help || len(os.Args) == 1 {
		printUsage()
		os.Exit(0)
	}

	if *decode {
		if err := decodeToHex(os.Stdin, os.Stdout, *useSVG); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := encodeHexToImage(os.Stdin, os.Stdout, *blocksPerRow, *useSVG); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding: %v\n", err)
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "  Encode: cat hexfile.txt | "+filepath.Base(os.Args[0])+" -b blocks_per_row [-v] > output.png/svg")
	fmt.Fprintln(os.Stderr, "  Decode: cat input.png/svg | "+filepath.Base(os.Args[0])+" -d [-v] > output.txt")
	fmt.Fprintln(os.Stderr, "\nOptions:")
	flag.PrintDefaults()
}

func encodeHexToImage(r io.Reader, w io.Writer, blocksPerRow int, useSVG bool) error {
	hexData, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	cleanHexData := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, string(hexData))

	data, err := hex.DecodeString(cleanHexData)
	if err != nil {
		return fmt.Errorf("decoding hex: %w", err)
	}

	blockCount := (len(data) + 2) / 3
	if blocksPerRow <= 0 {
		blocksPerRow = blockCount
	}

	rows := int(math.Ceil(float64(blockCount) / float64(blocksPerRow)))
	width := blocksPerRow * pixelSize
	height := rows * pixelSize

	if useSVG {
		return encodeSVG(w, data, width, height, blocksPerRow)
	}
	return encodePNG(w, data, width, height, blocksPerRow)
}

func encodePNG(w io.Writer, data []byte, width, height, blocksPerRow int) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for i := 0; i < len(data); i += 3 {
		r, g, b := getColor(data, i)
		drawBlock(img, i/3, blocksPerRow, r, g, b)
	}

	return png.Encode(w, img)
}

func encodeSVG(w io.Writer, data []byte, width, height, blocksPerRow int) error {
	canvas := svg.New(w)
	canvas.Start(width, height)

	for i := 0; i < len(data); i += 3 {
		r, g, b := getColor(data, i)
		x, y := getBlockPosition(i/3, blocksPerRow)
		canvas.Rect(x, y, pixelSize, pixelSize, fmt.Sprintf("fill:#%02x%02x%02x", r, g, b))
	}

	canvas.End()
	return nil
}

func getColor(data []byte, i int) (r, g, b uint8) {
	r = data[i]
	if i+1 < len(data) {
		g = data[i+1]
	}
	if i+2 < len(data) {
		b = data[i+2]
	}
	return
}

func drawBlock(img *image.RGBA, blockIndex, blocksPerRow int, r, g, b uint8) {
	x, y := getBlockPosition(blockIndex, blocksPerRow)
	for dy := 0; dy < pixelSize; dy++ {
		for dx := 0; dx < pixelSize; dx++ {
			img.Set(x+dx, y+dy, color.RGBA{r, g, b, 255})
		}
	}
}

func getBlockPosition(blockIndex, blocksPerRow int) (x, y int) {
	return (blockIndex % blocksPerRow) * pixelSize, (blockIndex / blocksPerRow) * pixelSize
}

func decodeToHex(r io.Reader, w io.Writer, fromSVG bool) error {
	var data []byte
	var err error

	if fromSVG {
		data, err = decodeSVG(r)
	} else {
		data, err = decodePNG(r)
	}

	if err != nil {
		return err
	}

	// Remove padding
	for len(data) > 0 && data[len(data)-1] == 0 {
		data = data[:len(data)-1]
	}

	// Write hex data
	_, err = fmt.Fprintf(w, "%x", data)
	if err != nil {
		return err
	}

	// Add a newline at the end
	_, err = fmt.Fprintln(w)
	return err
}

func decodePNG(r io.Reader) ([]byte, error) {
	img, err := png.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decoding PNG: %w", err)
	}

	bounds := img.Bounds()
	width, height := bounds.Max.X, bounds.Max.Y

	var data []byte

	for y := 0; y < height; y += pixelSize {
		for x := 0; x < width; x += pixelSize {
			r, g, b, _ := img.At(x, y).RGBA()
			data = append(data, uint8(r>>8), uint8(g>>8), uint8(b>>8))
		}
	}

	return data, nil
}

func decodeSVG(r io.Reader) ([]byte, error) {
	var data []byte
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "fill:#") {
			colorStr := strings.Split(line, "fill:#")[1][:6]
			color, err := hex.DecodeString(colorStr)
			if err != nil {
				return nil, fmt.Errorf("decoding color in SVG: %w", err)
			}
			data = append(data, color...)
		}
	}
	return data, scanner.Err()
}

