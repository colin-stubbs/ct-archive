package main

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	ct "github.com/google/certificate-transparency-go"
	"filippo.io/sunlight"
	"filippo.io/torchwood"
	"golang.org/x/mod/sumdb/note"
	"golang.org/x/mod/sumdb/tlog"
)

func TestParseMirrorCheckpoint_unsigned(t *testing.T) {
	t.Parallel()
	var zero tlog.Hash
	c := torchwood.Checkpoint{
		Origin: "example.com/log",
		Tree:   tlog.Tree{N: 0, Hash: zero},
	}
	got, err := parseMirrorCheckpoint([]byte(c.String()), "unused.example", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.N != 0 || got.Origin != "example.com/log" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseMirrorCheckpoint_signed(t *testing.T) {
	t.Parallel()
	sig, err := base64.StdEncoding.DecodeString("BAMASDBGAiEAnaHGuwnyyHWvrfgEn3qtl1j2heMzocku6ZAItYD75m8CIQCpotlpH5GEPEfMMzky72BCuIl14FB65t5SWZ91vgTQOg==")
	if err != nil {
		t.Fatal(err)
	}
	sthBytes := []byte(`{
		"sha256_root_hash": "l+XrWXWRyp4SmATORgTfz4CcYS/VlE7CeTuWI0FAk3o=",
		"timestamp": 1588741228371,
		"tree_head_signature": "BAMASDBGAiEAnaHGuwnyyHWvrfgEn3qtl1j2heMzocku6ZAItYD75m8CIQCpotlpH5GEPEfMMzky72BCuIl14FB65t5SWZ91vgTQOg==",
		"tree_size": 90785920
	}`)
	var sth ct.SignedTreeHead
	if err := json.Unmarshal(sthBytes, &sth); err != nil {
		t.Fatal(err)
	}
	key, err := ct.PublicKeyFromB64("MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAESYlKFDLLFmA9JScaiaNnqlU8oWDytxIYMfswHy9Esg0aiX+WnP/yj4O0ViEHtLwbmOQeSWBGkIu9YK9CLeer+g==")
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := ct.NewSignatureVerifier(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifier.VerifySTHSignature(sth); err != nil {
		t.Fatal(err)
	}
	signer, err := sunlight.NewRFC6962InjectedSigner("example.com", key, sig, int64(sth.Timestamp))
	if err != nil {
		t.Fatal(err)
	}
	cp := torchwood.Checkpoint{
		Origin: "example.com",
		Tree: tlog.Tree{
			N:    int64(sth.TreeSize),
			Hash: tlog.Hash(sth.SHA256RootHash),
		},
	}
	signed, err := note.Sign(&note.Note{Text: cp.String()}, signer)
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseMirrorCheckpoint([]byte(signed), "example.com", key)
	if err != nil {
		t.Fatal(err)
	}
	if got.N != int64(sth.TreeSize) {
		t.Fatalf("tree size: got %d want %d", got.N, sth.TreeSize)
	}
}
