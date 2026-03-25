package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds runtime configuration from environment variables.
type Config struct {
	PublicBaseURL     string
	ListenAddr        string
	UploadDir         string
	OutputDir         string
	MaxUploadBytes    int64
	MaxDownloadBytes  int64
	HTTPFetchTimeout  time.Duration
	FileTTL           time.Duration
	RenderDPI         int
}

func getenv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func mustParseInt64(s string, def int64) int64 {
	if s == "" {
		return def
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func mustParseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

// Load reads configuration from the environment.
func Load() (*Config, error) {
	base := getenv("PUBLIC_BASE_URL", "")
	if base == "" {
		return nil, fmt.Errorf("PUBLIC_BASE_URL is required")
	}

	port := getenv("PORT", "8000")
	dpi := int(mustParseInt64(getenv("RENDER_DPI", "150"), 150))
	if dpi < 72 || dpi > 300 {
		dpi = 150
	}

	return &Config{
		PublicBaseURL:    base,
		ListenAddr:       ":" + port,
		UploadDir:        getenv("UPLOAD_DIR", "uploads"),
		OutputDir:        getenv("OUTPUT_DIR", "outputs"),
		MaxUploadBytes:   mustParseInt64(getenv("MAX_UPLOAD_BYTES", "33554432"), 33554432), // 32 MiB
		MaxDownloadBytes: mustParseInt64(getenv("MAX_DOWNLOAD_BYTES", "33554432"), 33554432),
		HTTPFetchTimeout: mustParseDuration(getenv("HTTP_FETCH_TIMEOUT", "120s"), 120*time.Second),
		FileTTL:          mustParseDuration(getenv("FILE_TTL", "1h"), time.Hour),
		RenderDPI:        dpi,
	}, nil
}
