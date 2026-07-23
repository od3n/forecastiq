package identity

import (
	"strings"
	"testing"
)

func TestGenerateKeyFormat(t *testing.T) {
	prefix, plaintext, err := generateKey()
	if err != nil {
		t.Fatalf("generateKey: %v", err)
	}
	if !strings.HasPrefix(prefix, keyPrefixLabel) {
		t.Errorf("prefix %q missing label %q", prefix, keyPrefixLabel)
	}
	if !strings.HasPrefix(plaintext, prefix+".") {
		t.Errorf("plaintext %q must start with %q.", plaintext, prefix)
	}
	got, ok := splitKey(plaintext)
	if !ok || got != prefix {
		t.Errorf("splitKey(%q) = %q,%v; want %q,true", plaintext, got, ok, prefix)
	}
	// Two generations differ.
	_, p2, _ := generateKey()
	if plaintext == p2 {
		t.Error("generateKey produced identical plaintexts")
	}
}

func TestSplitKeyRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", "nodot", "wrong_prefix.abc", "fiq_noseparator"} {
		if _, ok := splitKey(bad); ok {
			t.Errorf("splitKey(%q) = ok; want rejected", bad)
		}
	}
}

func TestHashVerifyRoundTrip(t *testing.T) {
	_, plaintext, err := generateKey()
	if err != nil {
		t.Fatalf("generateKey: %v", err)
	}
	hash, err := hashKey(plaintext)
	if err != nil {
		t.Fatalf("hashKey: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("hash %q is not PHC argon2id", hash)
	}
	if strings.Contains(hash, plaintext) {
		t.Error("hash must not contain the plaintext")
	}

	ok, err := verifyKey(plaintext, hash)
	if err != nil || !ok {
		t.Errorf("verifyKey(correct) = %v,%v; want true,nil", ok, err)
	}

	// Wrong secret and a tampered hash both fail (no false accept).
	if ok, _ := verifyKey(plaintext+"x", hash); ok {
		t.Error("verifyKey accepted a wrong secret")
	}
	if ok, err := verifyKey(plaintext, hash[:len(hash)-4]+"AAAA"); ok && err == nil {
		t.Error("verifyKey accepted a tampered hash")
	}
}

func TestVerifyKeyRejectsBadFormat(t *testing.T) {
	for _, bad := range []string{"", "plain", "$bcrypt$x$y", "$argon2id$v=19$bad"} {
		if ok, err := verifyKey("whatever", bad); ok || err == nil {
			t.Errorf("verifyKey(_, %q) = %v,%v; want false,error", bad, ok, err)
		}
	}
}

// TestHashSaltIsRandom ensures two hashes of the same input differ (per-hash salt).
func TestHashSaltIsRandom(t *testing.T) {
	h1, _ := hashKey("fiq_abc.secret")
	h2, _ := hashKey("fiq_abc.secret")
	if h1 == h2 {
		t.Error("expected distinct salts to yield distinct hashes")
	}
}
