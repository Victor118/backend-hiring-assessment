package domain

// AdDetails is a read model of an ad enriched with data owned by other services. Username is empty when it could not be resolved.
type AdDetails struct {
	Ad       Ad
	Username string
}
