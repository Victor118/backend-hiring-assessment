package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
)

// AdImport is a batch of ads imported at once by their owner. Its key identifies it across retries, so that retrying an
// import does not create its ads twice.
type AdImport struct {
	OwnerID UserID
	Key     string
	Drafts  []AdDraft
}

// Fingerprint identifies the content of the import: two imports with the same drafts, in the same order, share it. It
// tells a retry of an import apart from a reuse of its key for another content. The owner and the key are left out: they
// identify the import, its fingerprint only describes what it contains.
func (i AdImport) Fingerprint() []byte {
	// Marshalling a slice of plain structs cannot fail, and the fields are always encoded in the same order.
	payload, _ := json.Marshal(i.Drafts)
	sum := sha256.Sum256(payload)

	return sum[:]
}

// ImportRecord is what is stored for an import key: the fingerprint of the import made under it, and the identifiers
// of the ads it created, in the order of the import.
type ImportRecord struct {
	RequestHash []byte
	AdIDs       []AdID
}

// Replay answers a retry of the import with the identifiers stored under its key. previous must be the record stored
// under the same owner and key: the key is checked by the lookup, Replay checks the content. It returns
// ErrImportKeyReused when previous describes another content, so that a reused key never silently drops an import.
func (i AdImport) Replay(previous ImportRecord) ([]AdID, error) {
	if !bytes.Equal(i.Fingerprint(), previous.RequestHash) {
		return nil, ErrImportKeyReused
	}

	return previous.AdIDs, nil
}
