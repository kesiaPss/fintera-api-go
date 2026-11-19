package services

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	xwebp "golang.org/x/image/webp"
)

func makeTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func assertWebP(t *testing.T, data []byte) image.Image {
	t.Helper()
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		t.Fatalf("expected WebP container, got %q", data[:min(12, len(data))])
	}
	img, err := xwebp.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode webp: %v", err)
	}
	return img
}

func TestPhotoToWebPDownsizesFullAndThumbnail(t *testing.T) {
	full, thumb, err := PhotoToWebP(makeTestPNG(t, 4000, 2000))
	if err != nil {
		t.Fatalf("PhotoToWebP: %v", err)
	}

	fullImg := assertWebP(t, full)
	if b := fullImg.Bounds(); b.Dx() > PhotoMaxDimension || b.Dy() > PhotoMaxDimension {
		t.Fatalf("full image = %dx%d, exceeds %d", b.Dx(), b.Dy(), PhotoMaxDimension)
	}
	thumbImg := assertWebP(t, thumb)
	if b := thumbImg.Bounds(); b.Dx() > PhotoThumbnailDimension || b.Dy() > PhotoThumbnailDimension {
		t.Fatalf("thumbnail = %dx%d, exceeds %d", b.Dx(), b.Dy(), PhotoThumbnailDimension)
	}
}

func TestResizeToWebPDoesNotUpscaleSmallImages(t *testing.T) {
	out, err := ResizeToWebP(makeTestPNG(t, 10, 7), 2048, 80)
	if err != nil {
		t.Fatalf("ResizeToWebP: %v", err)
	}
	img := assertWebP(t, out)
	if b := img.Bounds(); b.Dx() != 10 || b.Dy() != 7 {
		t.Fatalf("small image resized to %dx%d, want 10x7", b.Dx(), b.Dy())
	}
}

func TestResizeToWebPRejectsNonImage(t *testing.T) {
	if _, err := ResizeToWebP([]byte("definitely not an image"), 100, 80); !errors.Is(err, ErrUnsupportedRaster) {
		t.Fatalf("expected ErrUnsupportedRaster, got %v", err)
	}
}

func TestLogoToWebPPreservesTransparency(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	out, err := LogoToWebP(buf.Bytes())
	if err != nil {
		t.Fatalf("LogoToWebP: %v", err)
	}
	decoded := assertWebP(t, out)

	_, _, _, opaqueAlpha := decoded.At(8, 8).RGBA()
	if opaqueAlpha < 0xf000 {
		t.Fatalf("expected opaque region, alpha = %d", opaqueAlpha)
	}
	_, _, _, transparentAlpha := decoded.At(56, 32).RGBA()
	if transparentAlpha > 0x1000 {
		t.Fatalf("expected transparent region, alpha = %d", transparentAlpha)
	}
}
