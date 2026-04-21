package wgcrypto

import (
	"encoding/base64"
	"testing"
)

func TestGenerateKeyPair_Success(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	if priv == "" {
		t.Fatal("private key is empty")
	}
	if pub == "" {
		t.Fatal("public key is empty")
	}

	privBytes, err := base64.StdEncoding.DecodeString(priv)
	if err != nil {
		t.Fatalf("private key is not valid base64: %v", err)
	}
	if len(privBytes) != 32 {
		t.Fatalf("private key length = %d, want 32", len(privBytes))
	}

	pubBytes, err := base64.StdEncoding.DecodeString(pub)
	if err != nil {
		t.Fatalf("public key is not valid base64: %v", err)
	}
	if len(pubBytes) != 32 {
		t.Fatalf("public key length = %d, want 32", len(pubBytes))
	}
}

func TestGenerateKeyPair_UniqueKeys(t *testing.T) {
	keys := make(map[string]bool)
	for i := 0; i < 10; i++ {
		priv, pub, _ := GenerateKeyPair()
		if keys[priv] {
			t.Fatal("duplicate private key generated")
		}
		if keys[pub] {
			t.Fatal("duplicate public key generated")
		}
		keys[priv] = true
		keys[pub] = true
	}
}

func TestGeneratePresharedKey_Success(t *testing.T) {
	key, err := GeneratePresharedKey()
	if err != nil {
		t.Fatalf("GeneratePresharedKey returned error: %v", err)
	}
	if key == "" {
		t.Fatal("preshared key is empty")
	}

	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("preshared key is not valid base64: %v", err)
	}
	if len(keyBytes) != 32 {
		t.Fatalf("preshared key length = %d, want 32", len(keyBytes))
	}
}

func TestGeneratePresharedKey_Unique(t *testing.T) {
	keys := make(map[string]bool)
	for i := 0; i < 10; i++ {
		key, _ := GeneratePresharedKey()
		if keys[key] {
			t.Fatal("duplicate preshared key generated")
		}
		keys[key] = true
	}
}
