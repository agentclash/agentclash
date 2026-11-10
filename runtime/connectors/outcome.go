package connectors

import "errors"

// Rejection reasons. A rejection proves that the remote did not run the
// request.
const (
	// RejectionNotSent means that the request did not leave AgentClash.
	RejectionNotSent = "not_sent"
	// RejectionRefused means that the remote refused the request with a
	// status that it gives before it starts work.
	RejectionRefused = "refused"
)

var (
	// ErrNotSent matches each NotSentError.
	ErrNotSent = errors.New("connector request was not sent")
	// ErrRefused matches each RefusedError.
	ErrRefused = errors.New("connector refused the request")
)

// NotSentError is a failure before the request is sent. The remote did not
// get the request. Error gives the text of the cause, and Unwrap gives the
// cause, so errors.Is and errors.As find the cause.
type NotSentError struct {
	Err error
}

func (e *NotSentError) Error() string {
	if e.Err == nil {
		return ErrNotSent.Error()
	}
	return e.Err.Error()
}

func (e *NotSentError) Unwrap() error { return e.Err }

// Is makes errors.Is(err, ErrNotSent) true.
func (e *NotSentError) Is(target error) bool { return target == ErrNotSent }

// RefusedError is a response with a refusal status. The remote did not start
// the work. The error holds only the status, never the response body. Error
// gives the text that a non-success response had before this type.
type RefusedError struct {
	Status int
}

func (e *RefusedError) Error() string { return errNonSuccessText }

// Is makes errors.Is(err, ErrRefused) true.
func (e *RefusedError) Is(target error) bool { return target == ErrRefused }

const errNonSuccessText = "connector returned a non-success response; execution outcome is unknown"

// refusedStatuses are the statuses that a remote gives before it starts
// work. Each other non-success status keeps the outcome unknown: a 3xx, a
// 408, a 409 (the remote can already have the run), a 425, a 499 and each
// 5xx.
var refusedStatuses = map[int]struct{}{
	400: {}, 401: {}, 403: {}, 404: {}, 405: {}, 406: {}, 411: {},
	413: {}, 414: {}, 415: {}, 422: {}, 429: {}, 431: {},
}

// IsRefusedStatus reports whether status proves that the remote did not
// start the work.
func IsRefusedStatus(status int) bool {
	_, ok := refusedStatuses[status]
	return ok
}

// Rejection reports whether err proves that the remote did not run the
// request. The reason is RejectionNotSent or RejectionRefused. The status is
// the HTTP status of a refusal, and 0 for a request that was not sent. When
// ok is false, the outcome is unknown.
func Rejection(err error) (reason string, status int, ok bool) {
	var refused *RefusedError
	if errors.As(err, &refused) {
		return RejectionRefused, refused.Status, true
	}
	var unsent *NotSentError
	if errors.As(err, &unsent) {
		return RejectionNotSent, 0, true
	}
	return "", 0, false
}

func notSent(err error) error {
	return &NotSentError{Err: err}
}
