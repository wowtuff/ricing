package service

import (
	"crypto/rand"
	"encoding/base64"
)

func newID(prefix string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(b)
}
