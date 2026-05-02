package qrcode

import (
	"bytes"
	"testing"
)

func TestGeneratePNG_ValidContent(t *testing.T) {
	png, err := GeneratePNG("https://example.com/config", 256)
	if err != nil {
		t.Fatalf("GeneratePNG(): %v", err)
	}
	if len(png) == 0 {
		t.Error("GeneratePNG returned empty bytes")
	}
	if !bytes.HasPrefix(png, []byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Errorf("output does not start with PNG magic bytes, got %x", png[:4])
	}
}

func TestGeneratePNG_MinimalInput(t *testing.T) {
	png, err := GeneratePNG("x", 128)
	if err != nil {
		t.Fatalf("GeneratePNG(): %v", err)
	}
	if len(png) == 0 {
		t.Error("GeneratePNG returned empty bytes for minimal input")
	}
	if !bytes.HasPrefix(png, []byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Errorf("output does not start with PNG magic bytes")
	}
}

func TestGeneratePNG_SmallSize(t *testing.T) {
	png, err := GeneratePNG("hello", 64)
	if err != nil {
		t.Fatalf("GeneratePNG(): %v", err)
	}
	if len(png) == 0 {
		t.Error("GeneratePNG returned empty bytes")
	}
	if !bytes.HasPrefix(png, []byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Errorf("output does not start with PNG magic bytes")
	}
}

func TestGeneratePNG_LargeContent(t *testing.T) {
	longURL := "https://example.com/very/long/path/that/contains/a/lot/of/characters/to/encode/into/the/qr/code/image"
	png, err := GeneratePNG(longURL, 512)
	if err != nil {
		t.Fatalf("GeneratePNG(): %v", err)
	}
	if len(png) == 0 {
		t.Error("GeneratePNG returned empty bytes for large content")
	}
	if !bytes.HasPrefix(png, []byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Errorf("output does not start with PNG magic bytes")
	}
}

func TestGeneratePNG_DifferentSizesProduceDifferentOutput(t *testing.T) {
	small, err := GeneratePNG("test", 64)
	if err != nil {
		t.Fatalf("GeneratePNG(64): %v", err)
	}
	large, err := GeneratePNG("test", 512)
	if err != nil {
		t.Fatalf("GeneratePNG(512): %v", err)
	}
	if len(small) == len(large) {
		t.Error("different sizes should produce different output lengths")
	}
}

func TestGeneratePNG_EmptyString(t *testing.T) {
	_, err := GeneratePNG("", 128)
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestGeneratePNG_ZeroSize(t *testing.T) {
	png, err := GeneratePNG("test", 0)
	if err != nil {
		t.Fatalf("GeneratePNG with zero size: %v", err)
	}
	if !bytes.HasPrefix(png, []byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Errorf("output does not start with PNG magic bytes")
	}
}

func TestGeneratePNG_NegativeSize(t *testing.T) {
	png, err := GeneratePNG("test", -1)
	if err != nil {
		t.Fatalf("GeneratePNG with negative size: %v", err)
	}
	if !bytes.HasPrefix(png, []byte{0x89, 0x50, 0x4E, 0x47}) {
		t.Errorf("output does not start with PNG magic bytes")
	}
}
