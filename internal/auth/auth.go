package auth

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var usernameRE = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)

func ValidateUsername(name string) error {
	if !usernameRE.MatchString(name) {
		return fmt.Errorf("username must be 3–16 characters: letters, numbers or underscore")
	}
	return nil
}

// OfflineUUID reproduces Java's UUID.nameUUIDFromBytes("OfflinePlayer:"+name).
func OfflineUUID(username string) string {
	sum := md5.Sum([]byte("OfflinePlayer:" + username))
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(sum[:])
	return strings.Join([]string{raw[0:8], raw[8:12], raw[12:16], raw[16:20], raw[20:32]}, "-")
}
