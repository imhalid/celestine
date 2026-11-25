//go:build !js

package mcpauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

// HelloPath is the endpoint where the MCP stdio bridge and server exchange nonces and proofs.
const HelloPath = "/mcp/hello"

// ProofScheme is the Authorization scheme used for nonce-bound HMAC requests.
const ProofScheme = "Celestine"

// NonceSize is the fixed byte length of client and server nonces.
const NonceSize = 32

// ChallengeLife is how long an unused server nonce stays valid.
const ChallengeLife = time.Minute

// ChallengeMax bounds the number of retained server nonces.
const ChallengeMax = 256

// Hello is the JSON payload exchanged on HelloPath.
type Hello struct {
	Nonce string `json:"nonce"`
	Proof string `json:"proof,omitempty"`
}

// Proof computes HMAC-SHA256 over the role ("server" or "client") and both nonces under token.
func Proof(token, role string, clientNonce, serverNonce []byte) []byte {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte("celestine-mcp " + role + "\x00"))
	m.Write(clientNonce)
	m.Write(serverNonce)
	return m.Sum(nil)
}

// Nonce returns a fresh random nonce of NonceSize bytes.
func Nonce() []byte {
	b := make([]byte, NonceSize)
	_, _ = rand.Read(b)
	return b
}

type challenge struct {
	client []byte
	at     time.Time
	used   bool
}

// Challenges tracks issued server nonces behind a mutex for one MCP HTTP server instance.
type Challenges struct {
	mu sync.Mutex
	m  map[string]*challenge
}

// Add registers a server nonce bound to clientNonce, evicting expired or oldest entries when full.
func (cs *Challenges) Add(server string, clientNonce []byte, now time.Time) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.m == nil {
		cs.m = map[string]*challenge{}
	}
	if len(cs.m) >= ChallengeMax {
		var unused, used string
		for k, c := range cs.m {
			switch {
			case !c.used && now.Sub(c.at) > ChallengeLife:
				delete(cs.m, k)
			case !c.used && (unused == "" || c.at.Before(cs.m[unused].at)):
				unused = k
			case c.used && (used == "" || c.at.Before(cs.m[used].at)):
				used = k
			}
		}
		if len(cs.m) >= ChallengeMax {
			if unused != "" {
				delete(cs.m, unused)
			} else {
				delete(cs.m, used)
			}
		}
	}
	cs.m[server] = &challenge{client: clientNonce, at: now}
}

// Len returns the number of currently retained server nonces.
func (cs *Challenges) Len() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return len(cs.m)
}

// Has reports whether server nonce server is currently retained.
func (cs *Challenges) Has(server string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.m[server] != nil
}

// Check verifies a client's hex-encoded proof for the issued hex server nonce under token.
func (cs *Challenges) Check(server, proof, token string, now time.Time) bool {
	sn, err := hex.DecodeString(server)
	if err != nil || len(sn) != NonceSize {
		return false
	}
	got, err := hex.DecodeString(proof)
	if err != nil {
		return false
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	c := cs.m[server]
	if c == nil {
		return false
	}
	if !c.used && now.Sub(c.at) > ChallengeLife {
		delete(cs.m, server)
		return false
	}
	if !hmac.Equal(got, Proof(token, "client", c.client, sn)) {
		return false
	}
	c.used, c.at = true, now
	return true
}

// ServeHello responds to a bridge's client nonce with a server nonce and HMAC proof.
func ServeHello(w http.ResponseWriter, r *http.Request, cs *Challenges, token string, now time.Time) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var in Hello
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	cn, err := hex.DecodeString(in.Nonce)
	if err != nil || len(cn) != NonceSize {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sn := Nonce()
	cs.Add(hex.EncodeToString(sn), cn, now)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Hello{
		Nonce: hex.EncodeToString(sn),
		Proof: hex.EncodeToString(Proof(token, "server", cn, sn)),
	})
}

// Authorized reports whether an Authorization header carries the bearer token or a valid handshake proof.
func Authorized(auth, token string, cs *Challenges, now time.Time) bool {
	if token == "" {
		return false
	}
	if rest, ok := strings.CutPrefix(auth, ProofScheme+" "); ok {
		server, proof, ok := strings.Cut(rest, " ")
		return ok && cs.Check(server, proof, token, now)
	}
	return subtle.ConstantTimeCompare([]byte(auth), []byte("Bearer "+token)) == 1
}
