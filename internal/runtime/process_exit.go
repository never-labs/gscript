package runtime

import "fmt"

// ProcessExitError is returned by process.exit/os.exit so hosts can choose
// whether to terminate the OS process, report the status, or catch it in tests.
type ProcessExitError struct {
	Code int
}

func (e *ProcessExitError) Error() string {
	return fmt.Sprintf("process exit %d", e.Code)
}
