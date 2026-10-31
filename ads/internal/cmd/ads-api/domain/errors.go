package domain

const (
	// ErrAdNotFound is returned when no ad matches the requested identifier.
	ErrAdNotFound sentinelError = "ad not found"
	// ErrUnknownOwner is returned when an ad references a user that does not exist.
	ErrUnknownOwner sentinelError = "unknown owner"
)

type sentinelError string

// Error implements the error interface.
func (e sentinelError) Error() string { return string(e) }
