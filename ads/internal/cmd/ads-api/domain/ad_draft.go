package domain

import (
	"fmt"
	"strings"
)

// AdDraft is an ad that is not created yet: it has neither an identifier nor a creation date, which the repository
// assigns when it creates the ad.
type AdDraft struct {
	Title      string
	PriceCents int64
	PhotoURL   string
	OwnerID    UserID
}

// NewAdDraft is the way to build an AdDraft: it enforces the rules every ad must follow, whatever the entry point.
// The title is trimmed and must not be empty, the price must not be negative (a free ad is allowed) and the ad must
// have an owner. It returns an error wrapping ErrInvalidAd otherwise.
func NewAdDraft(title string, priceCents int64, photoURL string, ownerID UserID) (AdDraft, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return AdDraft{}, fmt.Errorf("%w: title is empty", ErrInvalidAd)
	}

	if priceCents < 0 {
		return AdDraft{}, fmt.Errorf("%w: price is negative", ErrInvalidAd)
	}

	if ownerID == "" {
		return AdDraft{}, fmt.Errorf("%w: owner is missing", ErrInvalidAd)
	}

	return AdDraft{
		Title:      title,
		PriceCents: priceCents,
		PhotoURL:   photoURL,
		OwnerID:    ownerID,
	}, nil
}
