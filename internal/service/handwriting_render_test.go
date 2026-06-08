package service

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"testing"
)

func TestNewDefaultPNGStrokeRendererAppliesNewDefaults(t *testing.T) {
	renderer := NewDefaultPNGStrokeRenderer()

	// Wide input should be capped at max width
	img := renderTestPNG(t, renderer, []Stroke{
		{
			Points: []StrokePoint{
				{X: 0, Y: 0},
				{X: 10000, Y: 10},
			},
		},
	})

	if img.Bounds().Dy() != 768 {
		t.Errorf("expected height 768, got %d", img.Bounds().Dy())
	}
	if img.Bounds().Dx() != 2304 {
		t.Errorf("expected width 2304, got %d", img.Bounds().Dx())
	}
}

func TestPNGStrokeRendererRenderPNG(t *testing.T) {
	renderer := NewPNGStrokeRenderer(64, 192, 8)

	imageBytes, err := renderer.RenderPNG([]Stroke{
		{
			Points: []StrokePoint{
				{X: 10, Y: 10},
				{X: 40, Y: 40},
				{X: 50, Y: 12},
			},
		},
	})
	if err != nil {
		t.Fatalf("RenderPNG() error = %v", err)
	}

	img, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	if img.Bounds().Dx() != 80 || img.Bounds().Dy() != 64 {
		t.Fatalf("image size = %dx%d, want 80x64", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestPNGStrokeRendererRejectsEmptyStrokes(t *testing.T) {
	renderer := NewPNGStrokeRenderer(64, 192, 8)

	if _, err := renderer.RenderPNG(nil); err != ErrEmptyStrokes {
		t.Fatalf("RenderPNG(nil) error = %v, want %v", err, ErrEmptyStrokes)
	}
}

func TestPNGStrokeRendererExpandsWidthForHorizontalInput(t *testing.T) {
	renderer := NewPNGStrokeRenderer(64, 192, 8)

	img := renderTestPNG(t, renderer, []Stroke{
		{
			Points: []StrokePoint{
				{X: 0, Y: 0},
				{X: 100, Y: 20},
			},
		},
	})

	if img.Bounds().Dx() <= img.Bounds().Dy() {
		t.Fatalf("image size = %dx%d, want width > height", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestPNGStrokeRendererCapsWidth(t *testing.T) {
	renderer := NewPNGStrokeRenderer(64, 192, 8)

	img := renderTestPNG(t, renderer, []Stroke{
		{
			Points: []StrokePoint{
				{X: 0, Y: 0},
				{X: 1000, Y: 10},
			},
		},
	})

	if img.Bounds().Dx() != 192 {
		t.Fatalf("image width = %d, want max width 192", img.Bounds().Dx())
	}
}

func TestPNGStrokeRendererPreservesInkAspectRatio(t *testing.T) {
	renderer := NewPNGStrokeRenderer(512, 1536, 48)

	img := renderTestPNG(t, renderer, []Stroke{
		{
			Points: []StrokePoint{
				{X: 0, Y: 0},
				{X: 200, Y: 0},
				{X: 200, Y: 100},
				{X: 0, Y: 100},
				{X: 0, Y: 0},
			},
		},
	})

	ink := inkBounds(img)
	got := float64(ink.Dx()) / float64(ink.Dy())
	if math.Abs(got-2) > 0.1 {
		t.Fatalf("ink aspect ratio = %.2f, want approximately 2.0", got)
	}
}

func TestPNGStrokeRendererPreservesSeparatedMark(t *testing.T) {
	renderer := NewPNGStrokeRenderer(64, 192, 8)

	img := renderTestPNG(t, renderer, []Stroke{
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
	})

	if got := blackPixelComponentCount(img); got != 2 {
		t.Fatalf("black pixel component count = %d, want 2", got)
	}
}

func renderTestPNG(t *testing.T, renderer *PNGStrokeRenderer, strokes []Stroke) image.Image {
	t.Helper()

	imageBytes, err := renderer.RenderPNG(strokes)
	if err != nil {
		t.Fatalf("RenderPNG() error = %v", err)
	}

	img, err := png.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	return img
}

func inkBounds(img image.Image) image.Rectangle {
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if !isBlack(img, x, y) {
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

	return image.Rect(minX, minY, maxX+1, maxY+1)
}

func blackPixelComponentCount(img image.Image) int {
	bounds := img.Bounds()
	visited := make(map[image.Point]struct{})
	count := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			start := image.Pt(x, y)
			if !isBlack(img, x, y) {
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
					image.Pt(current.X-1, current.Y),
					image.Pt(current.X+1, current.Y),
					image.Pt(current.X, current.Y-1),
					image.Pt(current.X, current.Y+1),
				} {
					if !next.In(bounds) || !isBlack(img, next.X, next.Y) {
						continue
					}
					if _, ok := visited[next]; ok {
						continue
					}
					visited[next] = struct{}{}
					queue = append(queue, next)
				}
			}
		}
	}

	return count
}

func isBlack(img image.Image, x, y int) bool {
	r, g, b, _ := img.At(x, y).RGBA()
	return r == 0 && g == 0 && b == 0
}
