// Command handwriting_renderer rebuilds a stroke JSON into the server-side PNG
// so client Canvas output and server RenderPNG() can be compared for parity.
//
// Dev-only tool. See docs/todos/handwriting_rebuild_parity_verification.md.
//
//	go run ./cmd/dev/handwriting_renderer \
//	  -input tmp/handwriting-parity/handakuten/strokes.json \
//	  -output tmp/handwriting-parity/handakuten/server.png
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/lsj/copylingo/internal/service"
)

// parityInput mirrors the JSON exported by the Mini App debug block. Only
// strokes is consumed by the renderer; canvas dimensions and line width are
// carried for human-readable parity context.
type parityInput struct {
	CanvasWidth  int              `json:"canvas_width,omitempty"`
	CanvasHeight int              `json:"canvas_height,omitempty"`
	LineWidth    int              `json:"line_width,omitempty"`
	Strokes      []service.Stroke `json:"strokes"`
}

func main() {
	input := flag.String("input", "", "path to stroke JSON exported from the Mini App")
	output := flag.String("output", "", "path to write the server-rebuilt PNG")
	flag.Parse()

	if *input == "" || *output == "" {
		flag.Usage()
		log.Fatal("both -input and -output are required")
	}

	if err := run(*input, *output); err != nil {
		log.Fatalf("render handwriting parity PNG: %v", err)
	}
	log.Printf("wrote server-rebuilt PNG to %s", *output)
}

func run(inputPath, outputPath string) error {
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read input %s: %w", inputPath, err)
	}

	var in parityInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return fmt.Errorf("parse stroke JSON %s: %w", inputPath, err)
	}

	pngBytes, err := service.NewDefaultPNGStrokeRenderer().RenderPNG(in.Strokes)
	if err != nil {
		return fmt.Errorf("render PNG: %w", err)
	}

	if err := os.WriteFile(outputPath, pngBytes, 0o644); err != nil {
		return fmt.Errorf("write output %s: %w", outputPath, err)
	}
	return nil
}
