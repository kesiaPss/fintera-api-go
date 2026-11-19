package services

import (
	"fmt"
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

// ProcessAndSaveProfilePicture stores the profile picture as a WebP image and
// creates a WebP thumbnail. Both files are re-encoded from the uploaded raster
// image; the original upload is not kept.
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
