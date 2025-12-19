package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CalculateInstallerHash downloads an installer and calculates its SHA256 hash.
func CalculateInstallerHash(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent to avoid blocks
	req.Header.Set("User-Agent", "Relicta-WinGet-Plugin/1.0")

	client := &http.Client{
		Timeout: 10 * time.Minute, // Large installers may take time
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download installer: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, resp.Body); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	return strings.ToUpper(hex.EncodeToString(hash.Sum(nil))), nil
}

// CalculateHashFromBytes calculates SHA256 hash from bytes.
func CalculateHashFromBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}
