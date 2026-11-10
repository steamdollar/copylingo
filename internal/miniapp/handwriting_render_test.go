package miniapp

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"testing"
)

func TestRenderStrokesPNGSizesCanvasToInk(t *testing.T) {
	imageBytes, err := RenderStrokesPNG([]Stroke{
		{
			Points: []StrokePoint{
				{X: 10, Y: 10},
				{X: 40, Y: 40},
				{X: 50, Y: 12},
			},
		},
	})
	if err != nil {
		t.Fatalf(
			"RenderPNG() error = %v",
			err,
		)
	}

	canvas, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		t.Fatalf(
			"png.Decode() error = %v",
			err,
		)
	}
	// 40x30 ink scaled to the 624px draw height is 832px wide, plus 2x72 padding.
	if canvas.Bounds().Dx() != 976 || canvas.Bounds().Dy() != renderHeight {
		t.Fatalf(
			"image size = %dx%d, want 976x%d",
			canvas.Bounds().Dx(),
			canvas.Bounds().Dy(),
			renderHeight,
		)
	}
}

func TestRenderStrokesPNGRejectsEmptyStrokes(t *testing.T) {
	if _, err := RenderStrokesPNG(nil); err != ErrEmptyStrokes {
		t.Fatalf(
			"RenderPNG(nil) error = %v, want %v",
			err,
			ErrEmptyStrokes,
		)
	}
}

func TestRenderStrokesPNGExpandsWidthForHorizontalInput(t *testing.T) {
	canvas := renderTestPNG(
		t,
		[]Stroke{
			{
				Points: []StrokePoint{
					{X: 0, Y: 0},
					{X: 100, Y: 40},
				},
			},
		},
	)

	if canvas.Bounds().Dx() <= canvas.Bounds().Dy() || canvas.Bounds().Dx() >= renderMaxWidth {
		t.Fatalf(
			"image size = %dx%d, want height < width < max",
			canvas.Bounds().Dx(),
			canvas.Bounds().Dy(),
		)
	}
}

func TestRenderStrokesPNGCapsWidth(t *testing.T) {
	canvas := renderTestPNG(
		t,
		[]Stroke{
			{
				Points: []StrokePoint{
					{X: 0, Y: 0},
					{X: 1000, Y: 10},
				},
			},
		},
	)

	if canvas.Bounds().Dx() != renderMaxWidth || canvas.Bounds().Dy() != renderHeight {
		t.Fatalf(
			"image size = %dx%d, want %dx%d",
			canvas.Bounds().Dx(),
			canvas.Bounds().Dy(),
			renderMaxWidth,
			renderHeight,
		)
	}
}

func TestRenderStrokesPNGPreservesInkAspectRatio(t *testing.T) {
	canvas := renderTestPNG(
		t,
		[]Stroke{
			{
				Points: []StrokePoint{
					{X: 0, Y: 0},
					{X: 200, Y: 0},
					{X: 200, Y: 100},
					{X: 0, Y: 100},
					{X: 0, Y: 0},
				},
			},
		},
	)

	ink := inkBounds(canvas)
	got := float64(ink.Dx()) / float64(ink.Dy())
	if math.Abs(got-2) > 0.1 {
		t.Fatalf(
			"ink aspect ratio = %.2f, want approximately 2.0",
			got,
		)
	}
}

func TestRenderStrokesPNGPreservesSeparatedMark(t *testing.T) {
	canvas := renderTestPNG(
		t,
		[]Stroke{
			{
				Points: []StrokePoint{
					{X: 0, Y: 0},
					{X: 0, Y: 100},
				},
			},
			{
				Points: []StrokePoint{
					{X: 50, Y: 0},
				},
			},
		},
	)

	if got := blackPixelComponentCount(canvas); got != 2 {
		t.Fatalf(
			"black pixel component count = %d, want 2",
			got,
		)
	}
}

func TestRenderStrokesPNGKeepsBentStrokeConnected(t *testing.T) {
	// 한 stroke 안에서 급하게 꺾이는 연속 경로(ㄱ 모양). 꺾인 지점에서도 끊기면 안 된다.
	canvas := renderTestPNG(
		t,
		[]Stroke{
			{
				Points: []StrokePoint{
					{X: 0, Y: 0},
					{X: 200, Y: 0},
					{X: 200, Y: 200},
				},
			},
		},
	)

	if got := blackPixelComponentCount(canvas); got != 1 {
		t.Fatalf(
			"black pixel component count = %d, want 1 (bent stroke must stay connected)",
			got,
		)
	}
}

func TestRenderStrokesPNGDoesNotConnectSeparateStrokes(t *testing.T) {
	// 서로 떨어진 두 multi-point stroke. 서버가 획 사이를 임의로 잇지 않아야 한다.
	canvas := renderTestPNG(
		t,
		[]Stroke{
			{
				Points: []StrokePoint{
					{X: 0, Y: 0},
					{X: 0, Y: 200},
				},
			},
			{
				Points: []StrokePoint{
					{X: 200, Y: 0},
					{X: 200, Y: 200},
				},
			},
		},
	)

	if got := blackPixelComponentCount(canvas); got != 2 {
		t.Fatalf(
			"black pixel component count = %d, want 2 (separate strokes must not connect)",
			got,
		)
	}
}

func renderTestPNG(
	t *testing.T,
	strokes []Stroke,
) image.Image {
	t.Helper()

	imageBytes, err := RenderStrokesPNG(strokes)
	if err != nil {
		t.Fatalf(
			"RenderPNG() error = %v",
			err,
		)
	}

	canvas, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		t.Fatalf(
			"png.Decode() error = %v",
			err,
		)
	}
	return canvas
}

func inkBounds(canvas image.Image) image.Rectangle {
	bounds := canvas.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if !isBlack(
				canvas,
				x,
				y,
			) {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}

	return image.Rect(
		minX,
		minY,
		maxX+1,
		maxY+1,
	)
}

func blackPixelComponentCount(canvas image.Image) int {
	bounds := canvas.Bounds()
	visited := make(map[image.Point]struct{})
	count := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			start := image.Pt(
				x,
				y,
			)
			if !isBlack(
				canvas,
				x,
				y,
			) {
				continue
			}
			if _, ok := visited[start]; ok {
				continue
			}

			count++
			queue := []image.Point{start}
			visited[start] = struct{}{}
			for len(queue) > 0 {
				current := queue[0]
				queue = queue[1:]
				for _, next := range []image.Point{
					image.Pt(
						current.X-1,
						current.Y,
					),
					image.Pt(
						current.X+1,
						current.Y,
					),
					image.Pt(
						current.X,
						current.Y-1,
					),
					image.Pt(
						current.X,
						current.Y+1,
					),
				} {
					if !next.In(bounds) || !isBlack(
						canvas,
						next.X,
						next.Y,
					) {
						continue
					}
					if _, ok := visited[next]; ok {
						continue
					}
					visited[next] = struct{}{}
					queue = append(
						queue,
						next,
					)
				}
			}
		}
	}

	return count
}

func isBlack(
	canvas image.Image,
	x,
	y int,
) bool {
	red, green, blue, _ := canvas.At(
		x,
		y,
	).RGBA()
	return red == 0 && green == 0 && blue == 0
}
