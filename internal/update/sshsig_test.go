package update

import (
	"os"
	"strings"
	"testing"
)

// testdata holds signatures made by `ssh-keygen -Y sign` with a throwaway
// key (private half deleted), so this checks the real OpenSSH format.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestVerifySSHSig(t *testing.T) {
	key := string(fixture(t, "test_key.pub"))
	msg := fixture(t, "checksums.txt")
	sig := fixture(t, "checksums.txt.sig")

	if err := VerifySSHSig(key, "cintelis-release", msg, sig); err != nil {
		t.Fatalf("genuine signature rejected: %v", err)
	}

	tampered := []byte(strings.Replace(string(msg), "abc123", "abc124", 1))
	if err := VerifySSHSig(key, "cintelis-release", tampered, sig); err == nil {
		t.Error("tampered checksums accepted")
	}
	if err := VerifySSHSig(key, "cintelis-release", msg, fixture(t, "wrong-namespace.sig")); err == nil {
		t.Error("signature from another namespace accepted")
	}
	other := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl other"
	if err := VerifySSHSig(other, "cintelis-release", msg, sig); err == nil {
		t.Error("signature checked against the wrong key accepted")
	}
	for name, bad := range map[string][]byte{
		"empty":     nil,
		"not pem":   []byte("hello"),
		"truncated": sig[:len(sig)/2],
	} {
		if err := VerifySSHSig(key, "cintelis-release", msg, bad); err == nil {
			t.Errorf("%s signature accepted", name)
		}
	}
	if err := VerifySSHSig("ssh-rsa AAAAB3NzaC1yc2E x", "cintelis-release", msg, sig); err == nil {
		t.Error("non-ed25519 key accepted")
	}
}
