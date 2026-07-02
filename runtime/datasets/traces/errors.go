package traces

import "fmt"

type unsupportedPlatformError string

func (e unsupportedPlatformError) Error() string {
	return fmt.Sprintf("unsupported trace source platform %q", string(e))
}

func errUnsupportedPlatform(raw string) error {
	return unsupportedPlatformError(raw)
}
