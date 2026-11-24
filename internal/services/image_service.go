package services

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sjperalta/fintera-api/pkg/logger"
)

// ImageService handles image processing and storage.
type ImageService struct {
	uploadDir string
	root      *os.Root
}

// NewImageService creates a new image service rooted at uploadDir.
func NewImageService(uploadDir string) *ImageService {
	// Ensure upload directory exists
	if err := os.MkdirAll(uploadDir, 0750); err != nil {
		logger.Warn("failed to create upload directory", "error", err)
	}
	root, err := os.OpenRoot(uploadDir)
	if err != nil {
		logger.Warn("failed to open upload directory", "error", err)
	}
	return &ImageService{
		uploadDir: uploadDir,
		root:      root,
	}
}

// Image dimension budget for profile pictures. A compressed image can declare
// far more pixels than its bytes cost to transport; decoding it would expand
// into width*height*4 bytes of RGBA. The caps keep the worst-case decode under
// roughly 256 MB of pixel memory (4096²) and typical uploads far below that,
// while still accepting any sane photo (16 MP total).
const (
	maxAvatarDimensionPixels = 4096
	maxAvatarTotalPixels     = 16_000_000
)

// ProcessAndSaveProfilePicture stores the profile picture as a WebP image and
// creates a WebP thumbnail. Both files are re-encoded from the uploaded raster
// image; the original upload is not kept. The dimension gate runs before the
// full decode so a pixel bomb cannot expand into hundreds of MB of memory.
func (s *ImageService) ProcessAndSaveProfilePicture(file multipart.File, header *multipart.FileHeader) (originalPath, thumbnailPath string, err error) {
	if s.root == nil {
		return "", "", fmt.Errorf("almacenamiento de imágenes no disponible")
	}

	// Validate file type
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		return "", "", fmt.Errorf("formato de imagen no soportado (solo JPG/PNG/WebP)")
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return "", "", fmt.Errorf("error al leer archivo: %w", err)
	}

	// Validate dimensions BEFORE decoding the full bitmap: header-only decode
	// so a pixel bomb cannot expand into hundreds of MB of memory in the WebP
	// pipeline that follows.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", "", fmt.Errorf("error al leer la cabecera de la imagen: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 ||
		cfg.Width > maxAvatarDimensionPixels || cfg.Height > maxAvatarDimensionPixels ||
		int64(cfg.Width)*int64(cfg.Height) > maxAvatarTotalPixels {
		return "", "", fmt.Errorf("la imagen excede las dimensiones máximas permitidas (%d×%d px)", maxAvatarDimensionPixels, maxAvatarDimensionPixels)
	}

	original, thumbnail, err := ProfileToWebP(data)
	if err != nil {
		return "", "", err
	}

	// Generate unique filename
	filename := uuid.New().String()
	originalFilename := filename + ".webp"
	thumbFilename := filename + "_thumb.webp"

	// Create relative paths for DB/Response (assuming /uploads is served statically)
	originalRelPath := "/uploads/" + originalFilename
	thumbRelPath := "/uploads/" + thumbFilename

	if err := s.root.WriteFile(originalFilename, original, 0600); err != nil {
		return "", "", fmt.Errorf("error al guardar imagen original: %w", err)
	}
	if err := s.root.WriteFile(thumbFilename, thumbnail, 0600); err != nil {
		return "", "", fmt.Errorf("error al guardar thumbnail: %w", err)
	}

	// Add timestamp query param to bust the browser cache on replacement.
	ts := fmt.Sprintf("%d", time.Now().Unix())
	return originalRelPath + "?t=" + ts, thumbRelPath + "?t=" + ts, nil
}
