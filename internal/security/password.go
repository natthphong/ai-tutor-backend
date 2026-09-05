package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strings"
)

func Token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func Digest(v string) string { b := sha256.Sum256([]byte(v)); return hex.EncodeToString(b[:]) }
func Hash(password string) string {
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		panic(e)
	}
	key := argon2.IDKey([]byte(password), salt, 2, 64*1024, 2, 32)
	return fmt.Sprintf("argon2id$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}
func Verify(hash, password string) bool {
	p := strings.Split(hash, "$")
	if len(p) != 3 || p[0] != "argon2id" {
		return false
	}
	s, e := base64.RawStdEncoding.DecodeString(p[1])
	if e != nil || len(s) != 16 {
		return false
	}
	k, e := base64.RawStdEncoding.DecodeString(p[2])
	if e != nil || len(k) != 32 {
		return false
	}
	return subtle.ConstantTimeCompare(k, argon2.IDKey([]byte(password), s, 2, 64*1024, 2, 32)) == 1
}
