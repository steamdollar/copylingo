package service

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
)

var ErrEmptyStrokes = errors.New("empty handwriting strokes")

const (
	defaultHandwritingRenderHeight   = 768
	defaultHandwritingRenderMaxWidth = 2304
	defaultHandwritingRenderPadding  = 72
)

// StrokeRenderer converts vector strokes into the compact image sent to Gemini.
type StrokeRenderer interface {
	RenderPNG(strokes []Stroke) ([]byte, error)
}

type PNGStrokeRenderer struct {
	height   int
	maxWidth int
	padding  int
	brush    int
}

func NewDefaultPNGStrokeRenderer() *PNGStrokeRenderer {
	return NewPNGStrokeRenderer(
		defaultHandwritingRenderHeight,
		defaultHandwritingRenderMaxWidth,
		defaultHandwritingRenderPadding,
	)
}

func NewPNGStrokeRenderer(height, maxWidth, padding int) *PNGStrokeRenderer {
	if maxWidth < height {
		maxWidth = height
	}
	// 획 두께를 캔버스 크기에 비례시켜, 해상도를 올려도 stroke가 상대적으로 얇아지지 않게 한다.
	// (256→5, 512→10) 탁점/반탁점 같은 미세 마크가 다운스케일 후에도 살아남도록.
	brush := height / 51
	if brush < 3 {
		brush = 3
	}
	return &PNGStrokeRenderer{height: height, maxWidth: maxWidth, padding: padding, brush: brush}
}

func (r *PNGStrokeRenderer) RenderPNG(strokes []Stroke) ([]byte, error) {
	if len(strokes) == 0 {
		return nil, ErrEmptyStrokes
	}

	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	hasPoint := false

	for _, stroke := range strokes {
		for _, p := range stroke.Points {
			minX = math.Min(minX, p.X)
			minY = math.Min(minY, p.Y)
			maxX = math.Max(maxX, p.X)
			maxY = math.Max(maxY, p.Y)
			hasPoint = true
		}
	}
	if !hasPoint {
		return nil, ErrEmptyStrokes
	}

	width := math.Max(maxX-minX, 1)
	height := math.Max(maxY-minY, 1)
	canvasWidth := r.canvasWidth(width, height)
	img := image.NewRGBA(image.Rect(0, 0, canvasWidth, r.height))
	fill(img, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	drawWidth := float64(canvasWidth - r.padding*2)
	drawHeight := float64(r.height - r.padding*2)
	scale := math.Min(drawWidth/width, drawHeight/height)
	offsetX := (float64(canvasWidth) - width*scale) / 2
	offsetY := (float64(r.height) - height*scale) / 2

	black := color.RGBA{A: 255}
	for _, stroke := range strokes {
		for i, p := range stroke.Points {
			x := int(math.Round((p.X-minX)*scale + offsetX))
			y := int(math.Round((p.Y-minY)*scale + offsetY))
			if i == 0 {
				drawBrush(img, x, y, r.brush, black)
				continue
			}
			prev := stroke.Points[i-1]
			px := int(math.Round((prev.X-minX)*scale + offsetX))
			py := int(math.Round((prev.Y-minY)*scale + offsetY))
			drawLine(img, px, py, x, y, r.brush, black)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r *PNGStrokeRenderer) canvasWidth(contentWidth, contentHeight float64) int {
	drawHeight := float64(r.height - r.padding*2)
	width := int(math.Ceil(contentWidth/contentHeight*drawHeight)) + r.padding*2
	if width < r.height {
		return r.height
	}
	if width > r.maxWidth {
		return r.maxWidth
	}
	return width
}

func fill(img *image.RGBA, c color.RGBA) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func drawLine(img *image.RGBA, x0, y0, x1, y1, brush int, c color.RGBA) {
	dx := int(math.Abs(float64(x1 - x0)))
	dy := -int(math.Abs(float64(y1 - y0)))
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy

	for {
		drawBrush(img, x0, y0, brush, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func drawBrush(img *image.RGBA, cx, cy, radius int, c color.RGBA) {
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
				continue
			}
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= radius*radius {
				img.SetRGBA(x, y, c)
			}
		}
	}
}
