// Package mutation carries proof that a request did not reach admission.
// Missing proof must be treated as an uncertain acknowledgement by clients.
package mutation

import "errors"

type rejected struct{ error }

func (e rejected) Unwrap() error { return e.error }
func Reject(err error) error {
	if err == nil || IsRejected(err) {
		return err
	}
	return rejected{err}
}
func IsRejected(err error) bool { var r rejected; return errors.As(err, &r) }
