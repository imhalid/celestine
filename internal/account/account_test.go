package account

import (
	"bytes"
	"errors"
	"testing"
)

// The keys come from the name as the server takes it and the password: the same for the same, other for another
// password or stretching; the data key sealed with one opens with it alone (#634).
func TestKeysAndDataKey(t *testing.T) {
	kdf, err := NewKDF()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"", kdf} {
		a1, k1, err := Keys("Ali", "gizli parola", k)
		if err != nil {
			t.Fatal(err)
		}
		a2, k2, _ := Keys(" ali ", "gizli parola", k)
		a3, k3, _ := Keys("ali", "başka parola", k)
		if a1 != a2 || !bytes.Equal(k1, k2) || a1 == a3 || bytes.Equal(k1, k3) {
			t.Fatalf("kdf %q: keys not as the name and the password", k)
		}
		data := bytes.Repeat([]byte{9}, 32)
		w, err := WrapDataKey(k1, data)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := UnwrapDataKey(k1, w); err != nil || !bytes.Equal(got, data) {
			t.Fatalf("unwrapped %x, %v", got, err)
		}
		if _, err := UnwrapDataKey(k3, w); !errors.Is(err, ErrDecrypt) {
			t.Fatalf("another password's key opened it: %v", err)
		}
	}
	for _, bad := range []string{"argon2id$1$65536$1$00112233445566778899aabbccddeeff", "scrypt$1$2$3$aa", "argon2id$3$1048577$1$00112233445566778899aabbccddeeff"} {
		if _, _, err := Keys("ali", "p", bad); err == nil {
			t.Errorf("a stretching out of bounds taken: %q", bad)
		}
	}
	if RecoveryAuth(bytes.Repeat([]byte{7}, 32)) == RecoveryAuthOfBefore(bytes.Repeat([]byte{7}, 32)) {
		t.Fatal("the two recovery proofs are the same")
	}
}

// A client names its device as asked, and refuses a server that is not HTTPS.
func TestClient(t *testing.T) {
	c := New("", nil, func() string { return " Telefon " })
	if c.base != DefaultServer || c.device() != "Telefon" {
		t.Fatalf("%q %q", c.base, c.device())
	}
	if _, err := New("http://example.com", nil, nil).KDFOf("ali"); err == nil {
		t.Fatal("a server over HTTP spoken to")
	}
}
