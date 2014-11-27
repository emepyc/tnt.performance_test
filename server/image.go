package main

import (
	"bufio"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"

	"code.google.com/p/draw2d/draw2d"
)

// scale returns a function that transforms domain coordinates in range coordinates.
func scale(rng []float64, dmn []float64) func(float64) float64 {
	rangeFrom := rng[0]
	rangeTo := rng[1]
	domainFrom := dmn[0]
	domainTo := dmn[1]

	b := (rangeTo - rangeFrom) / (domainTo - domainFrom)
	// x' = (width / (from-to)) * (x-from)
	f := func(pos float64) float64 {
		// fmt.Printf("%d\n", float64(b*(float64(pos-domainFrom))))
		return float64(b * (pos - domainFrom))
	}

	return f
}

func drawTrack(data *ImgData) {
	gc := draw2d.NewGraphicContext(data.Img)
	fgColor := data.Track.FgColor
	// highColor := &Color{R: 0, G: 100, B: 0}
	// lowColor := &Color{R: 154, G: 205, B: 50}
	// boundColor := &Color{R: 205, G: 0, B: 0}
	vOffset := data.Track.VOffset
	trackOffset := data.Track.Height / 8
	upLim := vOffset + trackOffset
	downLim := vOffset + (data.Track.Height - (2 * trackOffset))

	// Draw the horizontal guides
	gc.MoveTo(0, upLim)
	gc.LineTo(data.Conf.Width, upLim)
	gc.MoveTo(0, downLim)
	gc.LineTo(data.Conf.Width, downLim)
	gc.SetStrokeColor(fgColor)
	gc.Stroke()

	// Draw the gaps blocks
	// for _, block := range data.NodeData.Gaps {
	// 	block_start := data.Scale(block.Start)
	// 	block_end := data.Scale(block.End)
	// 	draw2d.Rect(gc, block_start, (vOffset + trackOffset), block_end, vOffset+(data.Track.Height-(2*trackOffset)))

	// 	if block.Type == "low" {
	// 		gc.SetFillColor(lowColor)
	// 	} else if block.Type == "high" {
	// 		gc.SetFillColor(highColor)
	// 	} else {
	// 		panic("Unknown gap type: " + block.Type)
	// 	}
	// 	gc.Fill()
	// 	gc.Stroke()
	// }

	// Draw the exon boundaries
	// for _, bound := range data.NodeData.ExonBoundaries {
	// 	where := data.Scale(bound)
	// 	gc.MoveTo(where, vOffset)
	// 	gc.LineTo(where, vOffset+data.Track.Height)
	// 	gc.SetStrokeColor(boundColor)
	// 	gc.Stroke()
	// }

	// Draw sequences if zoomed enough
	// if (data.Loc.To - data.Loc.To) < 300 {

	return
}

func saveToPngFile(filePath string, m image.Image) {
	f, err := os.Create(filePath)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer f.Close()
	b := bufio.NewWriter(f)
	err = png.Encode(b, m)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	err = b.Flush()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %s OK.\n", filePath)
}
