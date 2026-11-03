package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdDraft(t *testing.T) {
	const (
		title    = "Mountain bike"
		price    = int64(12000)
		photoURL = "https://example.com/photos/bike.jpg"
		owner    = UserID("user-1")
	)

	t.Run("nominal", func(t *testing.T) {
		draft, err := NewAdDraft(title, price, photoURL, owner)

		require.NoError(t, err)
		assert.Equal(t, AdDraft{Title: title, PriceCents: price, PhotoURL: photoURL, OwnerID: owner}, draft)
	})

	t.Run("accepts a free ad", func(t *testing.T) {
		draft, err := NewAdDraft(title, 0, photoURL, owner)

		require.NoError(t, err)
		assert.Equal(t, int64(0), draft.PriceCents)
	})

	t.Run("trims the title", func(t *testing.T) {
		draft, err := NewAdDraft("  Mountain bike\t\n", price, photoURL, owner)

		require.NoError(t, err)
		assert.Equal(t, title, draft.Title)
	})

	invalid := map[string]struct {
		title      string
		priceCents int64
		ownerID    UserID
		reason     string
	}{
		"empty title":           {title: "", priceCents: price, ownerID: owner, reason: "title is empty"},
		"whitespace-only title": {title: " \t\n ", priceCents: price, ownerID: owner, reason: "title is empty"},
		"negative price":        {title: title, priceCents: -1, ownerID: owner, reason: "price is negative"},
		"missing owner":         {title: title, priceCents: price, ownerID: "", reason: "owner is missing"},
	}
	for name, tc := range invalid {
		t.Run("rejects an invalid ad: "+name, func(t *testing.T) {
			draft, err := NewAdDraft(tc.title, tc.priceCents, photoURL, tc.ownerID)

			require.ErrorIs(t, err, ErrInvalidAd)
			assert.ErrorContains(t, err, tc.reason)
			assert.Zero(t, draft)
		})
	}
}
