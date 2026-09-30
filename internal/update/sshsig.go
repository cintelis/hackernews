package update

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// VerifySSHSig checks an OpenSSH file signature (`ssh-keygen -Y sign`, the
// SSHSIG format) over msg. pubKey is an authorized_keys-style line,
// "ssh-ed25519 AAAA… comment"; the signature must be by exactly that key and
// in namespace ns. Only Ed25519 keys are accepted.
func VerifySSHSig(pubKey, ns string, msg, armored []byte) error {
	want, err := parseEd25519(pubKey)
	if err != nil {
		return fmt.Errorf("release key: %w", err)
	}
	block, _ := pem.Decode(armored)
	if block == nil || block.Type != "SSH SIGNATURE" {
		return errors.New("not an SSH signature")
	}
	r := wire{b: block.Bytes}
	if string(r.next(6)) != "SSHSIG" || r.uint32() != 1 {
		return errors.New("unsupported signature format")
	}
	pk, namespace, reserved, hashAlg, sig := r.string(), r.string(), r.string(), r.string(), r.string()
	if r.err != nil {
		return errors.New("malformed signature")
	}

	kr := wire{b: pk}
	if kt, key := kr.string(), kr.string(); kr.err != nil || string(kt) != "ssh-ed25519" || !bytes.Equal(key, want) {
		return errors.New("signed by a different key")
	}
	if string(namespace) != ns {
		return fmt.Errorf("signature namespace %q, want %q", namespace, ns)
	}
	var digest []byte
	switch string(hashAlg) {
	case "sha512":
		h := sha512.Sum512(msg)
		digest = h[:]
	case "sha256":
		h := sha256.Sum256(msg)
		digest = h[:]
	default:
		return fmt.Errorf("unsupported hash %q", hashAlg)
	}
	sr := wire{b: sig}
	if string(sr.string()) != "ssh-ed25519" {
		return errors.New("not an Ed25519 signature")
	}
	raw := sr.string()
	if sr.err != nil || len(raw) != ed25519.SignatureSize {
		return errors.New("malformed signature")
	}

	// what was signed: the magic preamble, then the fields, then H(msg)
	var signed bytes.Buffer
	signed.WriteString("SSHSIG")
	for _, f := range [][]byte{namespace, reserved, hashAlg, digest} {
		putString(&signed, f)
	}
	if !ed25519.Verify(want, signed.Bytes(), raw) {
		return errors.New("signature does not match")
	}
	return nil
}

func parseEd25519(line string) (ed25519.PublicKey, error) {
	f := strings.Fields(line)
	if len(f) < 2 || f[0] != "ssh-ed25519" {
		return nil, errors.New("not an ssh-ed25519 public key")
	}
	blob, err := base64.StdEncoding.DecodeString(f[1])
	if err != nil {
		return nil, err
	}
	r := wire{b: blob}
	if string(r.string()) != "ssh-ed25519" {
		return nil, errors.New("key type mismatch")
	}
	key := r.string()
	if r.err != nil || len(key) != ed25519.PublicKeySize {
		return nil, errors.New("malformed public key")
	}
	return ed25519.PublicKey(key), nil
}

// wire reads SSH wire-format fields; after a short read, err is set and
// every later read returns nil.
type wire struct {
	b   []byte
	err error
}

func (w *wire) next(n int) []byte {
	if w.err != nil {
		return nil
	}
	if n < 0 || len(w.b) < n {
		w.err, w.b = errors.New("short read"), nil
		return nil
	}
	out := w.b[:n]
	w.b = w.b[n:]
	return out
}

func (w *wire) uint32() uint32 {
	b := w.next(4)
	if b == nil {
		return 0
	}
	return binary.BigEndian.Uint32(b)
}

func (w *wire) string() []byte {
	n := w.uint32()
	if n > 1<<20 { // nothing in a signature comes close
		w.err = errors.New("field too long")
		return nil
	}
	return w.next(int(n))
}

func putString(b *bytes.Buffer, s []byte) {
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(s)))
	b.Write(n[:])
	b.Write(s)
}
