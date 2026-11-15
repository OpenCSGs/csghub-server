package token

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const cl100kVocabularyURL = "https://openaipublic.blob.core.windows.net/encodings/cl100k_base.tiktoken"

// SDK cache key is the SHA-1 of the public URL (a filename, not a security hash).
const cl100kCacheKey = "9b5ad71b2ce5302211f9c61530b329a4922fc6a4"
const cl100kDigest = "223921b76ee99bde995b7ff738513eef100fb51d18c93597a113bcffe865b2a7"

func ensureCl100kCache(ctx context.Context) error {
	cacheDir := strings.TrimSpace(os.Getenv("TIKTOKEN_CACHE_DIR"))
	if cacheDir == "" {
		cacheDir = strings.TrimSpace(os.Getenv("DATA_GYM_CACHE_DIR"))
	}
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "data-gym-cache")
	}
	cachePath := filepath.Join(cacheDir, cl100kCacheKey)
	if _, err := os.Stat(cachePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cl100kVocabularyURL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: tokenizerLoadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vocabulary HTTP status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != cl100kDigest {
		return fmt.Errorf("cl100k vocabulary digest mismatch")
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(cacheDir, "cl100k-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp.Name(), cachePath)
}
