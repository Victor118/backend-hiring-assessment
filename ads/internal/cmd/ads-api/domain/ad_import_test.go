package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAdImport(drafts ...AdDraft) AdImport {
	return AdImport{OwnerID: "user-1", Key: "key-1", Drafts: drafts}
}

var (
	bike = AdDraft{Title: "Mountain bike", PriceCents: 12000, PhotoURL: "https://example.com/photos/bike.jpg", OwnerID: "user-1"}
	sofa = AdDraft{Title: "Leather sofa", PriceCents: 25000, PhotoURL: "https://example.com/photos/sofa.jpg", OwnerID: "user-1"}
)

func TestAdImport_Fingerprint(t *testing.T) {
	t.Run("is a SHA-256 digest", func(t *testing.T) {
		assert.Len(t, newTestAdImport(bike, sofa).Fingerprint(), 32)
	})

	t.Run("is the same for the same content", func(t *testing.T) {
		assert.Equal(t, newTestAdImport(bike, sofa).Fingerprint(), newTestAdImport(bike, sofa).Fingerprint())
	})

	t.Run("ignores the owner and the key, which identify the import rather than describe it", func(t *testing.T) {
		other := AdImport{OwnerID: "user-2", Key: "key-2", Drafts: []AdDraft{bike, sofa}}

		assert.Equal(t, newTestAdImport(bike, sofa).Fingerprint(), other.Fingerprint())
	})

	t.Run("differs when an ad differs", func(t *testing.T) {
		cheaperSofa := sofa
		cheaperSofa.PriceCents = 20000

		assert.NotEqual(t, newTestAdImport(bike, sofa).Fingerprint(), newTestAdImport(bike, cheaperSofa).Fingerprint())
	})

	t.Run("differs when the ads come in another order", func(t *testing.T) {
		assert.NotEqual(t, newTestAdImport(bike, sofa).Fingerprint(), newTestAdImport(sofa, bike).Fingerprint())
	})
}

func TestAdImport_Replay(t *testing.T) {
	t.Run("returns the IDs of a previous import with the same content", func(t *testing.T) {
		adImport := newTestAdImport(bike, sofa)
		previous := ImportRecord{RequestHash: adImport.Fingerprint(), AdIDs: []AdID{10, 11}}

		adIDs, err := adImport.Replay(previous)

		require.NoError(t, err)
		assert.Equal(t, []AdID{10, 11}, adIDs)
	})

	t.Run("rejects a previous import with another content: the key was reused, not retried", func(t *testing.T) {
		adImport := newTestAdImport(bike, sofa)
		previous := ImportRecord{RequestHash: newTestAdImport(bike).Fingerprint(), AdIDs: []AdID{10}}

		adIDs, err := adImport.Replay(previous)

		require.ErrorIs(t, err, ErrImportKeyReused)
		assert.Nil(t, adIDs)
	})
}
