//go:build !js

package main

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

// How the bridge of `celestine mcp` and the app's server know each other without the token on the wire (#170).
// A process that took the server's port first would otherwise be handed the token by the next bridge. So the
// bridge sends a nonce of its own to mcpHelloPath, the server answers with a nonce of its own and
// HMAC(token, "server", both nonces), and only when that holds does the bridge prove itself with
// HMAC(token, "client", both nonces) on each request, under the server's nonce. Neither proof gives the token
// away, and a proof is good only on the server that made its nonce.
//
// An agent that speaks HTTP itself (the line `claude mcp add --transport http` of the settings) still sends
// "Authorization: Bearer <token>", which the server takes as before.

// mcpHelloPath is where the bridge and the server prove to each other that they hold the token.
const mcpHelloPath = "/mcp/hello"

// mcpProofScheme is the Authorization scheme of a request made with a proof: "Celestine <server nonce> <proof>".
const mcpProofScheme = "Celestine"

// mcpNonceSize is the length of each side's nonce, fixed so the bytes a proof is made of read only one way.
const mcpNonceSize = 32

// mcpChallengeLife is how long a server's nonce waits for its first request with a proof.
const mcpChallengeLife = time.Minute

// mcpChallengeMax is how many nonces the server keeps: a local process asking again and again does not grow them.
const mcpChallengeMax = 256

// mcpHello is the body of the handshake each way: the nonce, and from the server its proof.
type mcpHello struct {
	Nonce string `json:"nonce"`
	Proof string `json:"proof,omitempty"`
}

// mcpProof is HMAC-SHA256 under the token of the side's role ("server" or "client") and both nonces.
func mcpProof(token, role string, clientNonce, serverNonce []byte) []byte {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte("celestine-mcp " + role + "\x00"))
	m.Write(clientNonce)
	m.Write(serverNonce)
	return m.Sum(nil)
}

// mcpNonce is a new random nonce.
func mcpNonce() []byte {
	b := make([]byte, mcpNonceSize)
	rand.Read(b)
	return b
}

// mcpChallenge is a nonce the server gave: the client's it answered, when it was given or last used by a proof,
// and whether a proof has used it yet.
type mcpChallenge struct {
	client []byte
	at     time.Time
	used   bool
}

// mcpChallenges are the server's nonces, by their hex; each handler (a start of the server) has its own, so a
// stopped server's are gone with it. They are the handler's, behind their own lock, not the app's state.
type mcpChallenges struct {
	mu sync.Mutex
	m  map[string]*mcpChallenge
}

// add keeps a new server nonce for the client's, making room when there is none: nonces past their life first,
// then the oldest one no proof has used, and when every one is in use, the one used longest ago (its bridge, if
// it is still there, is refused and ends, as when the app restarts). Each bridge that ran leaves a used nonce, so
// a server that runs for weeks would otherwise fill up.
func (cs *mcpChallenges) add(server string, client []byte, now time.Time) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.m == nil {
		cs.m = map[string]*mcpChallenge{}
	}
	if len(cs.m) >= mcpChallengeMax {
		var unused, used string
		for k, c := range cs.m {
			switch {
			case !c.used && now.Sub(c.at) > mcpChallengeLife:
				delete(cs.m, k)
			case !c.used && (unused == "" || c.at.Before(cs.m[unused].at)):
				unused = k
			case c.used && (used == "" || c.at.Before(cs.m[used].at)):
				used = k
			}
		}
		if len(cs.m) >= mcpChallengeMax {
			if unused != "" {
				delete(cs.m, unused)
			} else {
				delete(cs.m, used)
			}
		}
	}
	cs.m[server] = &mcpChallenge{client: client, at: now}
}

// check says whether proof is the client's for the server nonce server under token; the first good one marks the
// nonce used, after which it no longer runs out, and each good one keeps when it was last used.
func (cs *mcpChallenges) check(server, proof, token string, now time.Time) bool {
	sn, err := hex.DecodeString(server)
	if err != nil || len(sn) != mcpNonceSize {
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
	if !c.used && now.Sub(c.at) > mcpChallengeLife {
		delete(cs.m, server)
		return false
	}
	if !hmac.Equal(got, mcpProof(token, "client", c.client, sn)) {
		return false
	}
	c.used, c.at = true, now
	return true
}

// mcpServeHello answers a bridge's nonce with the server's and its proof.
func mcpServeHello(w http.ResponseWriter, r *http.Request, cs *mcpChallenges, token string, now time.Time) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if token == "" { // a proof under no key proves nothing
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var in mcpHello
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	cn, err := hex.DecodeString(in.Nonce)
	if err != nil || len(cn) != mcpNonceSize {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sn := mcpNonce()
	cs.add(hex.EncodeToString(sn), cn, now)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mcpHello{Nonce: hex.EncodeToString(sn), Proof: hex.EncodeToString(mcpProof(token, "server", cn, sn))})
}

// mcpAuthorized says whether a request's Authorization holds the token (an agent over HTTP) or a proof made after
// the handshake (the bridge).
func mcpAuthorized(auth, token string, cs *mcpChallenges, now time.Time) bool {
	if token == "" {
		return false
	}
	if rest, ok := strings.CutPrefix(auth, mcpProofScheme+" "); ok {
		server, proof, ok := strings.Cut(rest, " ")
		return ok && cs.check(server, proof, token, now)
	}
	return subtle.ConstantTimeCompare([]byte(auth), []byte("Bearer "+token)) == 1
}
