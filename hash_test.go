package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCalculateInstallerHash(t *testing.T) {
	// Create test server
	testContent := []byte("test installer content")
	expectedHash := CalculateHashFromBytes(testContent)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(testContent)
	}))
	defer server.Close()

	hash, err := CalculateInstallerHash(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hash != expectedHash {
		t.Errorf("expected hash '%s', got '%s'", expectedHash, hash)
	}
}

func TestCalculateInstallerHashNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := CalculateInstallerHash(context.Background(), server.URL)
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

func TestCalculateInstallerHashRedirect(t *testing.T) {
	testContent := []byte("redirected content")
	expectedHash := CalculateHashFromBytes(testContent)

	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(testContent)
	}))
	defer finalServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL, http.StatusFound)
	}))
	defer redirectServer.Close()

	hash, err := CalculateInstallerHash(context.Background(), redirectServer.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hash != expectedHash {
		t.Errorf("expected hash '%s', got '%s'", expectedHash, hash)
	}
}

func TestCalculateHashFromBytes(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "empty",
			data:     []byte{},
			expected: "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855",
		},
		{
			name:     "hello",
			data:     []byte("hello"),
			expected: "2CF24DBA5FB0A30E26E83B2AC5B9E29E1B161E5C1FA7425E73043362938B9824",
		},
		{
			name:     "test content",
			data:     []byte("test installer content"),
			expected: "19EB2AA2B331FDAA7935E86424A3AA04BAF374AD7DE0DDDB57D5F0F3B7030934",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateHashFromBytes(tt.data)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestCalculateInstallerHashInvalidURL(t *testing.T) {
	_, err := CalculateInstallerHash(context.Background(), "http://invalid.nonexistent.url.test/file.exe")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}
