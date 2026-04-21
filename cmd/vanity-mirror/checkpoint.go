package main

import (
	"fmt"

	"filippo.io/sunlight"
	"filippo.io/torchwood"
	"golang.org/x/mod/sumdb/note"
)

// parseMirrorCheckpoint decodes a vanity-mirror checkpoint file. While a run is
// in progress the file is an unsigned torchwood checkpoint; when the mirror
// finishes it is replaced with a signed note (same as photocamera-archiver). The
// unsigned form is accepted directly; the signed form is verified with the log
// public key and the inner text is parsed.
func parseMirrorCheckpoint(data []byte, origin string, pubKey any) (torchwood.Checkpoint, error) {
	c, err := torchwood.ParseCheckpoint(string(data))
	if err == nil {
		return c, nil
	}
	unsignedErr := err
	v, err := sunlight.NewRFC6962Verifier(origin, pubKey)
	if err != nil {
		return torchwood.Checkpoint{}, fmt.Errorf("parse checkpoint: %w", unsignedErr)
	}
	n, err := note.Open(data, note.VerifierList(v))
	if err != nil {
		return torchwood.Checkpoint{}, fmt.Errorf("parse unsigned checkpoint: %w; verify signed checkpoint: %w", unsignedErr, err)
	}
	c, err = torchwood.ParseCheckpoint(n.Text)
	if err != nil {
		return torchwood.Checkpoint{}, fmt.Errorf("parse checkpoint text from signed note: %w", err)
	}
	return c, nil
}
