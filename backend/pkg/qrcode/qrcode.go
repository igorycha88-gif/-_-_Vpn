package qrcode

import (
	"fmt"

	"github.com/skip2/go-qrcode"
)

func GeneratePNG(content string, size int) ([]byte, error) {
	png, err := qrcode.Encode(content, qrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("qrcode.GeneratePNG: %w", err)
	}
	return png, nil
}
