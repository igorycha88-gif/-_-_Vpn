package wgcrypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

func GenerateKeyPair() (privateKey, publicKey string, err error) {
	var priv [32]byte
	_, err = rand.Read(priv[:])
	if err != nil {
		return "", "", fmt.Errorf("wgcrypto.GenerateKeyPair: read random: %w", err)
	}

	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	var pub [32]byte
	curve25519.ScalarBaseMult(&pub, &priv)

	return encodeKey(priv[:]), encodeKey(pub[:]), nil
}

func GeneratePresharedKey() (string, error) {
	var key [32]byte
	_, err := rand.Read(key[:])
	if err != nil {
		return "", fmt.Errorf("wgcrypto.GeneratePresharedKey: %w", err)
	}
	return encodeKey(key[:]), nil
}

func encodeKey(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
