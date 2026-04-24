package service

import (
	"bytes"
	"image/png"
	"testing"
)

func TestPNGStrokeRendererRenderPNG(t *testing.T) {
	renderer := NewPNGStrokeRenderer(64, 8)

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
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		t.Fatalf("image size = %dx%d, want 64x64", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestPNGStrokeRendererRejectsEmptyStrokes(t *testing.T) {
	renderer := NewPNGStrokeRenderer(64, 8)

	if _, err := renderer.RenderPNG(nil); err != ErrEmptyStrokes {
		t.Fatalf("RenderPNG(nil) error = %v, want %v", err, ErrEmptyStrokes)
	}
}
