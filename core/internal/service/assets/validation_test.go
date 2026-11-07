package assets

import (
	"context"
	"image"
	"image/png"
	"os"
	"testing"
)

func TestImageValidation(t *testing.T) {
	for _, test := range []struct {
		name          string
		width, height int
		mime          string
		valid         bool
	}{
		{"product image", 32, 24, "image/png", true}, {"wrong type", 32, 24, "image/jpeg", false}, {"oversized dimensions", 8193, 1, "image/png", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "image-")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if err = png.Encode(file, image.NewRGBA(image.Rect(0, 0, test.width, test.height))); err != nil {
				t.Fatal(err)
			}
			mime, width, height, err := validateFile(context.Background(), file, test.mime)
			if (err == nil) != test.valid {
				t.Fatalf("validation err=%v", err)
			}
			if test.valid && (mime != test.mime || width != test.width || height != test.height) {
				t.Fatal("image metadata mismatch")
			}
		})
	}
}

func TestActiveContentIsRejected(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "active-")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	file.WriteString(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	if _, _, _, err := validateFile(context.Background(), file, "image/png"); err == nil {
		t.Fatal("SVG accepted as an image upload")
	}
}
