package out

import (
	"fmt"
	"strings"
)

// UsageError is a command called wrongly: exit 2, the command line naming
// the help after its message.
type UsageError struct{ message string }

func (e UsageError) Error() string { return e.message }

// Usage is a usage error saying the formatted text, trimmed.
func Usage(format string, a ...any) error {
	return UsageError{strings.TrimSpace(fmt.Sprintf(format, a...))}
}
