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
		for _, point := range stroke.Points {
			minX = math.Min(
				minX,
				point.X,
			)
			minY = math.Min(
				minY,
				point.Y,
			)
			maxX = math.Max(
				maxX,
				point.X,
			)
			maxY = math.Max(
				maxY,
				point.Y,
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
	canvas := image.NewRGBA(image.Rect(
		0,
		0,
		canvasWidth,
		renderHeight,
	))
	fill(
		canvas,
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
		for i, point := range stroke.Points {
			x := int(math.Round((point.X-minX)*scale + offsetX))
			y := int(math.Round((point.Y-minY)*scale + offsetY))
			if i == 0 {
				drawBrush(
					canvas,
					x,
					y,
					renderBrush,
					black,
				)
				continue
			}
			previousPoint := stroke.Points[i-1]
			previousPixelX := int(math.Round((previousPoint.X-minX)*scale + offsetX))
			previousPixelY := int(math.Round((previousPoint.Y-minY)*scale + offsetY))
			drawLine(
				canvas,
				previousPixelX,
				previousPixelY,
				x,
				y,
				renderBrush,
				black,
			)
		}
	}

	var buffer bytes.Buffer
	if err := png.Encode(
		&buffer,
		canvas,
	); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
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
	canvas *image.RGBA,
	fillColor color.RGBA,
) {
	for y := canvas.Bounds().Min.Y; y < canvas.Bounds().Max.Y; y++ {
		for x := canvas.Bounds().Min.X; x < canvas.Bounds().Max.X; x++ {
			canvas.SetRGBA(
				x,
				y,
				fillColor,
			)
		}
	}
}

func drawLine(
	canvas *image.RGBA,
	currentX,
	currentY,
	targetX,
	targetY,
	brush int,
	lineColor color.RGBA,
) {
	deltaX := int(math.Abs(float64(targetX - currentX)))
	deltaY := -int(math.Abs(float64(targetY - currentY)))
	stepX := -1
	if currentX < targetX {
		stepX = 1
	}
	stepY := -1
	if currentY < targetY {
		stepY = 1
	}
	errorTerm := deltaX + deltaY

	for {
		drawBrush(
			canvas,
			currentX,
			currentY,
			brush,
			lineColor,
		)
		if currentX == targetX && currentY == targetY {
			break
		}
		doubledErrorTerm := 2 * errorTerm
		if doubledErrorTerm >= deltaY {
			errorTerm += deltaY
			currentX += stepX
		}
		if doubledErrorTerm <= deltaX {
			errorTerm += deltaX
			currentY += stepY
		}
	}
}

func drawBrush(
	canvas *image.RGBA,
	centerX,
	centerY,
	radius int,
	brushColor color.RGBA,
) {
	for y := centerY - radius; y <= centerY+radius; y++ {
		for x := centerX - radius; x <= centerX+radius; x++ {
			if x < 0 || y < 0 || x >= canvas.Bounds().Dx() || y >= canvas.Bounds().Dy() {
				continue
			}
			if (x-centerX)*(x-centerX)+(y-centerY)*(y-centerY) <= radius*radius {
				canvas.SetRGBA(
					x,
					y,
					brushColor,
				)
			}
		}
	}
}
