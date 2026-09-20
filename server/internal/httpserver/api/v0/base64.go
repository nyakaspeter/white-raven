package v0

import "encoding/base64"

func decodeURLBase64(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}
