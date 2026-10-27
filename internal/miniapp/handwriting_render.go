package miniapp

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
)

var ErrEmptyStrokes = errors.New("empty handwriting strokes")

// The PNG sent to Gemini is always renderHeight tall; its width follows the ink
// aspect ratio within [renderHeight, renderMaxWidth]. Sized so dakuten and small
// kana survive the model's downscaling (ADR_from_01_to_20.md, 2026-06).
const (
	renderHeight   = 768
	renderMaxWidth = 2304
	renderPadding  = 72
	// Brush scales with the canvas so strokes keep their relative thickness.
	renderBrush = renderHeight / 51
)

// StrokePoint represents one sampled drawing point from the Mini App canvas.
type StrokePoint struct {
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	TimeMS  int64   `json:"time_ms,omitempty"`
	Drawing bool    `json:"drawing"`
}

// Stroke represents one continuous pen-down path.
type Stroke struct {
	Points []StrokePoint `json:"points"`
}

// RenderStrokesPNG converts the Mini App's vector strokes into the compact image
// sent to Gemini. The ink bounding box is cropped and scaled to renderHeight, so
// the client canvas size does not affect the result.
func RenderStrokesPNG(strokes []Stroke) ([]byte, error) {
	if len(strokes) == 0 {
		return nil, ErrEmptyStrokes
	}

	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	hasPoint := false

	for _, stroke := range strokes {
		for _, p := range stroke.Points {
			minX = math.Min(
				minX,
				p.X,
			)
			minY = math.Min(
				minY,
				p.Y,
			)
			maxX = math.Max(
				maxX,
				p.X,
			)
			maxY = math.Max(
				maxY,
				p.Y,
			)
			hasPoint = true
		}
	}
	if !hasPoint {
		return nil, ErrEmptyStrokes
	}

	width := math.Max(
		maxX-minX,
		1,
	)
	height := math.Max(
		maxY-minY,
		1,
	)
	canvasWidth := renderCanvasWidth(
		width,
		height,
	)
	img := image.NewRGBA(image.Rect(
		0,
		0,
		canvasWidth,
		renderHeight,
	))
	fill(
		img,
		color.RGBA{R: 255, G: 255, B: 255, A: 255},
	)

	drawWidth := float64(canvasWidth - renderPadding*2)
	drawHeight := float64(renderHeight - renderPadding*2)
	scale := math.Min(
		drawWidth/width,
		drawHeight/height,
	)
	offsetX := (float64(canvasWidth) - width*scale) / 2
	offsetY := (float64(renderHeight) - height*scale) / 2

	black := color.RGBA{A: 255}
	for _, stroke := range strokes {
		for i, p := range stroke.Points {
			x := int(math.Round((p.X-minX)*scale + offsetX))
			y := int(math.Round((p.Y-minY)*scale + offsetY))
			if i == 0 {
				drawBrush(
					img,
					x,
					y,
					renderBrush,
					black,
				)
				continue
			}
			prev := stroke.Points[i-1]
			px := int(math.Round((prev.X-minX)*scale + offsetX))
			py := int(math.Round((prev.Y-minY)*scale + offsetY))
			drawLine(
				img,
				px,
				py,
				x,
				y,
				renderBrush,
				black,
			)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(
		&buf,
		img,
	); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderCanvasWidth(
	contentWidth,
	contentHeight float64,
) int {
	drawHeight := float64(renderHeight - renderPadding*2)
	width := int(math.Ceil(contentWidth/contentHeight*drawHeight)) + renderPadding*2
	if width < renderHeight {
		return renderHeight
	}
	if width > renderMaxWidth {
		return renderMaxWidth
	}
	return width
}

func fill(
	img *image.RGBA,
	c color.RGBA,
) {
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.SetRGBA(
				x,
				y,
				c,
			)
		}
	}
}

func drawLine(
	img *image.RGBA,
	x0,
	y0,
	x1,
	y1,
	brush int,
	c color.RGBA,
) {
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
		drawBrush(
			img,
			x0,
			y0,
			brush,
			c,
		)
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

func drawBrush(
	img *image.RGBA,
	cx,
	cy,
	radius int,
	c color.RGBA,
) {
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
				continue
			}
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= radius*radius {
				img.SetRGBA(
					x,
					y,
					c,
				)
			}
		}
	}
}
