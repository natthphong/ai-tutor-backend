package security

import "testing"

func TestPasswordHash(t *testing.T) {
	a, b := Hash("correct horse battery"), Hash("correct horse battery")
	if a == b {
		t.Fatal("salts reused")
	}
	if !Verify(a, "correct horse battery") || Verify(a, "wrong") {
		t.Fatal("verification failed")
	}
	for _, h := range []string{"", "argon2id$bad$bad", "bcrypt$a$b"} {
		if Verify(h, "anything") {
			t.Fatal("accepted invalid hash")
		}
	}
}
func TestTokens(t *testing.T) {
	a, b := Token(), Token()
	if len(a) < 40 || a == b || Digest(a) == a {
		t.Fatal("invalid token")
	}
}
