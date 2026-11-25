//go:build !js

package main

import (
	"net/http"
	"time"

	"github.com/imhalid/celestine/internal/mcpauth"
)

// Root wrappers/aliases into internal/mcpauth so callers in main and existing tests stay concise.
const (
	mcpHelloPath     = mcpauth.HelloPath
	mcpProofScheme   = mcpauth.ProofScheme
	mcpNonceSize     = mcpauth.NonceSize
	mcpChallengeLife = mcpauth.ChallengeLife
	mcpChallengeMax  = mcpauth.ChallengeMax
)

type (
	mcpHello      = mcpauth.Hello
	mcpChallenges = mcpauth.Challenges
)

var (
	mcpProof = mcpauth.Proof
	mcpNonce = mcpauth.Nonce
)

func mcpServeHello(w http.ResponseWriter, r *http.Request, cs *mcpChallenges, token string, now time.Time) {
	mcpauth.ServeHello(w, r, cs, token, now)
}

func mcpAuthorized(auth, token string, cs *mcpChallenges, now time.Time) bool {
	return mcpauth.Authorized(auth, token, cs, now)
}
