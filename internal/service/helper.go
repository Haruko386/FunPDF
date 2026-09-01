package service

import (
	"FunPDF/conf"
	"FunPDF/internal/dto"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

/* file */

// saveJSON writes data atomically: temp file + rename.
func saveJSON(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".editor-state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

/* Album */

const (
	MaxImageSize = 4 * 1024 * 1024
)

// ValidateBase64ImageSize check image size is legal
func ValidateBase64ImageSize(base64Str string) error {
	if base64Str == "" {
		return fmt.Errorf("image is empty")
	}

	// remove data:image/png;base64 prefix
	imageData := base64Str
	if strings.HasPrefix(base64Str, "data:") {
		var found bool
		_, imageData, found = strings.Cut(base64Str, ",")
		if !found {
			return fmt.Errorf("invalid image data URI")
		}
	}

	// is image valid?
	decoded, err := base64.StdEncoding.DecodeString(imageData)
	if err != nil {
		return fmt.Errorf("base64 decode failed: %v", err)
	}

	// is image bigger than rule?
	if len(decoded) > MaxImageSize {
		return fmt.Errorf("image should not bigger than %d", len(decoded))
	}

	// is image format valid?
	if !isValidImageFormat(decoded) {
		return fmt.Errorf("image format is invalid")
	}

	return nil
}

// isValidImageFormat check image format
func isValidImageFormat(data []byte) bool {
	if len(data) < 12 {
		return false
	}

	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return true
	}

	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return true
	}

	// GIF: 47 49 46
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return true
	}

	// WebP: 52 49 46 46
	if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		return true
	}

	return false
}

func checkDuplicateIDs(ids []string) []string {
	validIDs := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))

	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		validIDs = append(validIDs, id)
	}
	return validIDs
}

func GetLocalJsonProviders() ([]dto.ListProvidersResult, error) {
	list := make([]dto.ListProvidersResult, 0)
	configs, err := conf.ReadModelConfigs()
	if err != nil {
		return nil, err
	}

	for _, data := range configs {
		var model dto.ListProvidersResult
		err = json.Unmarshal(data, &model)
		if err != nil {
			return nil, err
		}
		list = append(list, model)
	}

	return list, nil
}

func ExtractPDFText(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	totalPage := r.NumPage()
	var text strings.Builder
	for pageIndex := 1; pageIndex <= totalPage; pageIndex++ {
		page := r.Page(pageIndex)
		if page.V.IsNull() {
			continue
		}
		content, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		text.WriteString(content)
		text.WriteString("\n")
	}
	return text.String(), nil
}
