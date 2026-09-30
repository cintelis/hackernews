// mkicns turns the CISO AI mark into the macOS app icon:
//
//	cd tools/mkicns && go run . ../../assets/cisoai-mark.png ../../internal/macapp/AppIcon.icns
//
// An .icns file is a list of (type, PNG) entries, one per size macOS asks
// for. It's a separate module so its image-resizing dependency stays out of
// the app; the generated file is committed.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"

	"golang.org/x/image/draw"
)

var entries = []struct {
	kind string
	size int
}{
	{"icp4", 16}, {"icp5", 32}, {"icp6", 64}, {"ic07", 128}, {"ic08", 256}, {"ic09", 512},
	{"ic11", 32}, {"ic12", 64}, {"ic13", 256}, {"ic14", 512}, // the @2x (Retina) sizes
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: mkicns in.png out.icns")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	check(err)
	src, err := png.Decode(f)
	check(err)
	f.Close()

	var body bytes.Buffer
	for _, e := range entries {
		dst := image.NewNRGBA(image.Rect(0, 0, e.size, e.size))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
		var p bytes.Buffer
		check(png.Encode(&p, dst))
		body.WriteString(e.kind)
		check(binary.Write(&body, binary.BigEndian, uint32(8+p.Len())))
		body.Write(p.Bytes())
	}
	var out bytes.Buffer
	out.WriteString("icns")
	check(binary.Write(&out, binary.BigEndian, uint32(8+body.Len())))
	out.Write(body.Bytes())
	check(os.WriteFile(os.Args[2], out.Bytes(), 0o644))
	fmt.Printf("wrote %s (%d bytes, %d images)\n", os.Args[2], out.Len(), len(entries))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
