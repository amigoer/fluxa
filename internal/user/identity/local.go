package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

// GenerateOTP and HashOTP back the local-account fallback (DESIGN.md
// 7.1): phone/email login and registration both work by proving you
// received a short-lived code, matching the login screen design -- there
// is no password anywhere in the product.

// GenerateOTP returns a 6-digit numeric code. Low entropy on its own is
// fine here: the real defenses are a short expiry and single use,
// enforced by Repo.ConsumeOTP, not the code's keyspace.
func GenerateOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// HashOTP hashes a code for storage/comparison so the raw code is never
// persisted.
func HashOTP(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
