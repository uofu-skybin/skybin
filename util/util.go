package util

import (
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base32"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"crypto/sha256"
	"encoding/hex"
)

func Hash(data []byte) string {
	h := sha1.New()
	h.Write(data)
	return base32.StdEncoding.EncodeToString(h.Sum(nil))
}

func SaveJson(filename string, v interface{}) error {
	bytes, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, bytes, 0666)
}

func LoadJson(filename string, v interface{}) error {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, v)
}

func MarshalPrivateKey(key *rsa.PrivateKey) []byte {
	block := pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return pem.EncodeToMemory(&block)
}

func UnmarshalPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("Expected PEM formatted key bytes.")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func MarshalPublicKey(key *rsa.PublicKey) ([]byte, error) {
	bytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, err
	}
	block := pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: bytes,
	}
	return pem.EncodeToMemory(&block), nil
}

func UnmarshalPublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("Expected PEM formated key bytes.")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("Key is not an RSA public key.")
	}
	return key, nil
}

func FingerprintKey(key []byte) string {
	shaSum := sha256.Sum256(key)
	fingerprint := hex.EncodeToString(shaSum[:])
	return fingerprint
}

// Formats a byte amount as a string.
func FormatByteAmount(bytes int64) string {
	var amt float64
	var units string
	if bytes >= 1e12 {
		amt = float64(bytes) / float64(1e12)
		units = "TB"
	} else if bytes >= 1e9 {
		amt = float64(bytes) / float64(1e9)
		units = "GB"
	} else if bytes >= 1e6 {
		amt = float64(bytes) / float64(1e6)
		units = "MB"
	} else if bytes >= 1e3 {
		amt = float64(bytes) / float64(1e3)
		units = "KB"
	} else {
		amt = float64(bytes)
		units = "B"
	}
	return fmt.Sprintf("%.2f %s", amt, units)
}

// Parses a byte amount from a string.
// The string may include a suffix.
//    e.g. 10GB -> 10 * 1e9
func ParseByteAmount(str string) (int64, error) {

	// Normalize the string
	str = strings.TrimSpace(str)
	str = strings.ToLower(str)

	mul := int64(1)
	if strings.HasSuffix(str, "tb") {
		mul = 1e12
		str = str[:len(str)-len("tb")]
	} else if strings.HasSuffix(str, "gb") {
		mul = 1e9
		str = str[:len(str)-len("gb")]
	} else if strings.HasSuffix(str, "mb") {
		mul = 1e6
		str = str[:len(str)-len("gb")]
	} else if strings.HasSuffix(str, "kb") {
		mul = 1e3
		str = str[:len(str)-len("kb")]
	} else if strings.HasSuffix(str, "b") {
		str = str[:len(str)-len("b")]
	}

	// Trim any whitespace that preceded the byte prefix
	str = strings.TrimSpace(str)

	n, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, err
	}
	return n * mul, nil
}

// Validates a that a network address string is
// in host:port form.
func ValidateNetAddr(addr string) error {
	_, _, err := net.SplitHostPort(addr)
	return err
}

func GetTokenClaimsFromRequest(r *http.Request) (jwt.MapClaims, error) {
	// If the user is authenticated as the specified renter, return the entire object.
	user := r.Context().Value("user")
	token, ok := user.(*jwt.Token)
	if !ok {
		return nil, errors.New("could not get token from request")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("token claims in incorrect format")
	}

	return claims, nil
}

// Removes a single leading and trailing slash from a file name.
func CleanPath(path string) string {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	return path
}

