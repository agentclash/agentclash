package api

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrInvalidSlug = errors.New("invalid slug")

	slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

func validateSlug(slug string) error {
	if len(slug) < 3 || len(slug) > 60 {
		return fmt.Errorf("%w: slug must be between 3 and 60 characters", ErrInvalidSlug)
	}
	if !slugPattern.MatchString(slug) {
		return fmt.Errorf("%w: slug must be lowercase alphanumeric with hyphens, no leading or trailing hyphens", ErrInvalidSlug)
	}
	return nil
}
