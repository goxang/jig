package host

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/blake2b"
)

// OfficialKey is the minisign public key goxang's artifacts are signed with; empty in builds without one.
var OfficialKey = ""

// verifyMinisign checks a minisign signature (legacy Ed or prehashed ED) of data by pubkey.
func verifyMinisign(pubkey, sig string, data []byte) error {
	pk, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pubkey))
	if err != nil || len(pk) != 42 || string(pk[:2]) != "Ed" {
		return errors.New("bad minisign public key")
	}
	lines := strings.Split(strings.TrimSpace(sig), "\n")
	if len(lines) < 4 {
		return errors.New("bad minisign signature")
	}
	s, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[1]))
	if err != nil || len(s) != 74 {
		return errors.New("bad minisign signature")
	}
	if !bytes.Equal(s[2:10], pk[2:10]) {
		return errors.New("signed by a different key")
	}
	msg := data
	switch string(s[:2]) {
	case "ED":
		h := blake2b.Sum512(data)
		msg = h[:]
	case "Ed":
	default:
		return errors.New("unknown minisign algorithm")
	}
	key := ed25519.PublicKey(pk[10:])
	if !ed25519.Verify(key, msg, s[10:]) {
		return errors.New("signature does not match")
	}
	trusted, ok := strings.CutPrefix(strings.TrimSpace(lines[2]), "trusted comment: ")
	global, err := base64.StdEncoding.DecodeString(strings.TrimSpace(lines[3]))
	if !ok || err != nil || !ed25519.Verify(key, append(s[10:74:74], trusted...), global) {
		return errors.New("trusted comment signature does not match")
	}
	return nil
}

// publisherKey is the key artifacts of publisher must be signed with: goxang's built in, any other
// trusted on first use and pinned in ~/.config/jig/keys.json.
func publisherKey(publisher, offered string) (string, error) {
	if publisher == "goxang" {
		return OfficialKey, nil
	}
	keys := map[string]string{}
	file := filepath.Join(ConfigDir(), "keys.json")
	if b, err := os.ReadFile(file); err == nil {
		_ = json.Unmarshal(b, &keys)
	}
	if k, ok := keys[publisher]; ok {
		if offered != "" && offered != k {
			return "", fmt.Errorf("publisher %s now signs with a different key; if that is expected, remove it from %s", publisher, file)
		}
		return k, nil
	}
	if offered == "" {
		return "", nil
	}
	keys[publisher] = offered
	b, _ := json.MarshalIndent(keys, "", "  ")
	if err := os.MkdirAll(ConfigDir(), 0o700); err != nil {
		return "", err
	}
	return offered, writeAtomic(file, b, 0o600)
}
