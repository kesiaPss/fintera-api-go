package services

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readSeekCloser adapts a bytes.Reader to the multipart.File interface.
type readSeekCloser struct {
	*bytes.Reader
}

func (r readSeekCloser) Close() error { return nil }

func multipartFile(data []byte) readSeekCloser {
	return readSeekCloser{bytes.NewReader(data)}
}

func fileHeader(name string, size int) *multipart.FileHeader {
	return &multipart.FileHeader{
		Filename: name,
		Header:   textproto.MIMEHeader{},
		Size:     int64(size),
	}
}

// pngHeaderWithDimensions builds the minimal bytes image.DecodeConfig needs to
// read (signature + IHDR), declaring the given pixel dimensions without any
// image data. This lets the tests exercise huge dimensions cheaply.
func pngHeaderWithDimensions(t *testing.T, width, height int) []byte {
	t.Helper()

	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], uint32(width))
	binary.BigEndian.PutUint32(ihdr[4:8], uint32(height))
	ihdr[8] = 8 // bit depth
	ihdr[9] = 6 // color type RGBA

	body := &bytes.Buffer{}
	body.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	body.Write([]byte{0, 0, 0, 13})
	body.WriteString("IHDR")
	body.Write(ihdr)

	crc := crc32.NewIEEE()
	_, err := crc.Write([]byte("IHDR"))
	require.NoError(t, err)
	_, err = crc.Write(ihdr)
	require.NoError(t, err)
	crcBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(crcBytes, crc.Sum32())
	body.Write(crcBytes)

	return body.Bytes()
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	return len(entries)
}

func TestProcessAndSaveProfilePictureRejectsPixelBomb(t *testing.T) {
	uploadDir := t.TempDir()
	svc := NewImageService(uploadDir)

	// 5000×5000 declared pixels exceed both the side (4096) and the total
	// (16M) budget while the byte payload stays tiny.
	data := pngHeaderWithDimensions(t, 5000, 5000)

	_, _, err := svc.ProcessAndSaveProfilePicture(multipartFile(data), fileHeader("bomb.png", len(data)))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "excede las dimensiones máximas")
	assert.Zero(t, countFiles(t, uploadDir), "nothing must be written for a rejected image")
}

func TestProcessAndSaveProfilePictureRejectsOversizedPixelBudget(t *testing.T) {
	uploadDir := t.TempDir()
	svc := NewImageService(uploadDir)

	// Both sides are within the 4096 cap, but 4096×4000 = 16,384,000 pixels
	// exceeds the 16M total budget.
	data := pngHeaderWithDimensions(t, 4096, 4000)

	_, _, err := svc.ProcessAndSaveProfilePicture(multipartFile(data), fileHeader("big.png", len(data)))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "excede las dimensiones")
	assert.Zero(t, countFiles(t, uploadDir))
}

func TestProcessAndSaveProfilePictureAcceptsValidImage(t *testing.T) {
	uploadDir := t.TempDir()
	svc := NewImageService(uploadDir)

	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.NRGBA{255, 255, 255, 255})
		}
	}
	buf := &bytes.Buffer{}
	require.NoError(t, png.Encode(buf, img))

	originalPath, thumbPath, err := svc.ProcessAndSaveProfilePicture(
		multipartFile(buf.Bytes()), fileHeader("avatar.png", buf.Len()),
	)
	require.NoError(t, err)

	// Profile pictures are re-encoded as WebP (full + thumbnail) on main's
	// image pipeline; the originals are not kept.
	assert.True(t, strings.HasSuffix(trimQuery(originalPath), ".webp"))
	assert.Contains(t, thumbPath, "_thumb.webp")

	originalName := trimQuery(originalPath)[len("/uploads/"):]
	thumbName := trimQuery(thumbPath)[len("/uploads/"):]

	for _, name := range []string{originalName, thumbName} {
		data, err := os.ReadFile(filepath.Join(uploadDir, name))
		require.NoError(t, err)
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		require.NoErrorf(t, err, "stored file %s must be a decodable image", name)
		assert.Equalf(t, "webp", format, "stored file %s must be WebP", name)
		assert.Greater(t, cfg.Width, 0)
		assert.Greater(t, cfg.Height, 0)
	}

	thumbData, err := os.ReadFile(filepath.Join(uploadDir, thumbName))
	require.NoError(t, err)
	thumbCfg, _, err := image.DecodeConfig(bytes.NewReader(thumbData))
	require.NoError(t, err)
	assert.LessOrEqual(t, thumbCfg.Width, ProfileThumbnailDimension,
		"thumbnail stays within the avatar dimension")
}

// trimQuery removes a URL query string (e.g. "?t=123") from a stored path.
func trimQuery(path string) string {
	if idx := strings.Index(path, "?"); idx >= 0 {
		return path[:idx]
	}
	return path
}
