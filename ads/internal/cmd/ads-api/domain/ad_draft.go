package domain

// AdDraft is an ad that is not created yet: it has neither an identifier nor a creation date, which the repository
// assigns when it creates the ad.
type AdDraft struct {
	Title      string
	PriceCents int64
	PhotoURL   string
	OwnerID    UserID
}
