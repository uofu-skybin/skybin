package authorization

import (
	"crypto/rand"
	"encoding/base64"
)

func randString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
