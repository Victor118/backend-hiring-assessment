package domain

const (
	// ErrAdNotFound is returned when no ad matches the requested identifier.
	ErrAdNotFound sentinelError = "ad not found"
	// ErrUnknownOwner is returned when an ad references a user that does not exist.
	ErrUnknownOwner sentinelError = "unknown owner"
	// ErrNotOwner is returned when a user acts on an ad they do not own.
	ErrNotOwner sentinelError = "user is not owner"
	// ErrUserNotFound is returned when the users service does not know a user.
	ErrUserNotFound sentinelError = "user not found"
	// ErrInvalidAd is returned when an ad breaks a business rule. The wrapping error names the faulty field.
	ErrInvalidAd sentinelError = "invalid ad"
)

type sentinelError string

// Error implements the error interface.
func (e sentinelError) Error() string { return string(e) }
