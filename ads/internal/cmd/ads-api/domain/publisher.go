package domain

import "time"

const (
	// PublicationQuota is the number of ads a publisher may publish through POST /ads over PublicationWindow.
	PublicationQuota = 5
	// PublicationWindow is the rolling period the quota applies to.
	PublicationWindow = 24 * time.Hour
)

// Publisher is a user seen as the author of the ads it publishes. Its recent publications bound how many more ads it
// may publish: the quota applies to the publications of the last 24 hours.
type Publisher struct {
	ID UserID
	// RecentPublications is the number of ads published over the last 24 hours. It is only reliable while the
	// publisher is locked, so that no concurrent publication can change it.
	RecentPublications int
}

// AuthorizePublication returns ErrQuotaExceeded when the publisher has already reached its quota.
func (p Publisher) AuthorizePublication() error {
	if p.RecentPublications >= PublicationQuota {
		return ErrQuotaExceeded
	}

	return nil
}
