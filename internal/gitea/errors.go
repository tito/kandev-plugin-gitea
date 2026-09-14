package gitea

import "fmt"

type ErrorCode string

const (
	ErrUnauthenticated  ErrorCode = "unauthenticated"
	ErrPermissionDenied ErrorCode = "permission_denied"
	ErrNotFound         ErrorCode = "not_found"
	ErrConflict         ErrorCode = "conflict"
	ErrRateLimited      ErrorCode = "rate_limited"
	ErrUpstream         ErrorCode = "upstream"
)

type Error struct {
	Code       ErrorCode
	StatusCode int
	Message    string
}

func (e *Error) Error() string {
	return fmt.Sprintf("gitea: %s (HTTP %d): %s", e.Code, e.StatusCode, e.Message)
}

func mapHTTPError(statusCode int, message string) *Error {
	var code ErrorCode
	switch {
	case statusCode == 401:
		code = ErrUnauthenticated
	case statusCode == 403:
		code = ErrPermissionDenied
	case statusCode == 404:
		code = ErrNotFound
	case statusCode == 409:
		code = ErrConflict
	case statusCode == 429:
		code = ErrRateLimited
	case statusCode >= 500:
		code = ErrUpstream
	default:
		code = ErrUpstream
	}
	return &Error{Code: code, StatusCode: statusCode, Message: message}
}
