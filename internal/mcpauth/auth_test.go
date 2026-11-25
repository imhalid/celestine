//go:build !js

package mcpauth

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandshakeAndAuthorization(t *testing.T) {
	var cs Challenges
	token := "secret-token"
	now := time.Unix(1_700_000_000, 0)
	cn := Nonce()

	reqBody, _ := json.Marshal(Hello{Nonce: hex.EncodeToString(cn)})
	req := httptest.NewRequest(http.MethodPost, HelloPath, bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	ServeHello(rec, req, &cs, token, now)
	if rec.Code != http.StatusOK {
		t.Fatalf("ServeHello code = %d", rec.Code)
	}
	var out Hello
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	sn, _ := hex.DecodeString(out.Nonce)
	clientProof := hex.EncodeToString(Proof(token, "client", cn, sn))
	auth := ProofScheme + " " + out.Nonce + " " + clientProof

	if !Authorized(auth, token, &cs, now) {
		t.Fatal("expected valid proof authorization")
	}
	if Authorized(auth, "wrong-token", &cs, now) {
		t.Fatal("expected wrong token to fail")
	}
	if !Authorized("Bearer "+token, token, &cs, now) {
		t.Fatal("expected Bearer token authorization")
	}
	if Authorized("Bearer "+token, "", &cs, now) {
		t.Fatal("expected empty token to fail")
	}
}
