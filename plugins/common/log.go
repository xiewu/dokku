package common

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
)

// ErrWithExitCode wraps error and exposes an ExitCode method
type ErrWithExitCode interface {
	ExitCode() int
}

type writer struct {
	mu     *sync.Mutex
	source string
}

// Write prints the data to either stdout or stderr using the log helper functions
func (w *writer) Write(bytes []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.source == "stdout" {
		for _, line := range strings.Split(string(bytes), "\n") {
			if line == "" {
				continue
			}

			LogVerboseQuiet(line)
		}
	} else {
		for _, line := range strings.Split(string(bytes), "\n") {
			if line == "" {
				continue
			}

			LogVerboseStderrQuiet(line)
		}
	}

	return len(bytes), nil
}

// PrefixingWriter is a writer that prefixes all writes with a given prefix
type PrefixingWriter struct {
	Prefix []byte
	Writer io.Writer
}

// Write writes the given bytes to the writer with the prefix
func (pw *PrefixingWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	// Perform an "atomic" write of a prefix and p to make sure that it doesn't interleave
	// sub-line when used concurrently with io.PipeWrite.
	n, err := pw.Writer.Write(append(pw.Prefix, p...))
	if n > len(p) {
		// To comply with the io.Writer interface requirements we must
		// return a number of bytes written from p (0 <= n <= len(p)),
		// so we are ignoring the length of the prefix here.
		return len(p), err
	}
	return n, err
}

// LogFail is the failure log formatter
// prints text to stderr and exits with status 1
func LogFail(text string) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(" !     %s", text))
	os.Exit(1)
}

// LogFailWithError is the failure log formatter
// prints text to stderr and exits with the specified exit code
func LogFailWithError(err error) {
	if err == nil {
		return
	}

	if merr, ok := err.(*multierror.Error); ok {
		for _, e := range merr.Errors {
			fmt.Fprintf(os.Stderr, " !     %s\n", e.Error())
		}
	} else {
		fmt.Fprintf(os.Stderr, " !     %s\n", err.Error())
	}
	if errExit, ok := err.(ErrWithExitCode); ok {
		os.Exit(errExit.ExitCode())
	}
	os.Exit(1)
}

// LogFailWithErrorQuiet is the failure log formatter (with quiet option)
// prints text to stderr and exits with the specified exit code
// The error message is not printed if DOKKU_QUIET_OUTPUT has any value
func LogFailWithErrorQuiet(err error) {
	if os.Getenv("DOKKU_QUIET_OUTPUT") == "" {
		fmt.Fprintln(os.Stderr, fmt.Sprintf(" !     %s", err.Error()))
	}
	if errExit, ok := err.(ErrWithExitCode); ok {
		os.Exit(errExit.ExitCode())
	}
	os.Exit(1)
}

// LogFailQuiet is the failure log formatter (with quiet option)
// prints text to stderr and exits with status 1
func LogFailQuiet(text string) {
	if os.Getenv("DOKKU_QUIET_OUTPUT") == "" {
		fmt.Fprintln(os.Stderr, fmt.Sprintf(" !     %s", text))
	}
	os.Exit(1)
}

// Log is the log formatter
func Log(text string) {
	fmt.Println(text)
}

// LogQuiet is the log formatter (with quiet option)
func LogQuiet(text string) {
	if os.Getenv("DOKKU_QUIET_OUTPUT") == "" {
		fmt.Println(text)
	}
}

// LogInfo1 is the info1 header formatter
func LogInfo1(text string) {
	fmt.Println(fmt.Sprintf("-----> %s", text))
}

// LogInfo1Quiet is the info1 header formatter (with quiet option)
func LogInfo1Quiet(text string) {
	if os.Getenv("DOKKU_QUIET_OUTPUT") == "" {
		LogInfo1(text)
	}
}

// LogInfo2 is the info2 header formatter
func LogInfo2(text string) {
	fmt.Println(fmt.Sprintf("=====> %s", text))
}

// LogInfo2Quiet is the info2 header formatter (with quiet option)
func LogInfo2Quiet(text string) {
	if os.Getenv("DOKKU_QUIET_OUTPUT") == "" {
		LogInfo2(text)
	}
}

// LogVerbose is the verbose log formatter
// prints indented text to stdout
func LogVerbose(text string) {
	fmt.Println(fmt.Sprintf("       %s", text))
}

// LogVerboseStderr is the verbose log formatter
// prints indented text to stderr
func LogVerboseStderr(text string) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(" !     %s", text))
}

// LogVerboseQuiet is the verbose log formatter
// prints indented text to stdout (with quiet option)
func LogVerboseQuiet(text string) {
	if os.Getenv("DOKKU_QUIET_OUTPUT") == "" {
		LogVerbose(text)
	}
}

// LogVerboseStderrQuiet is the verbose log formatter
// prints indented text to stderr (with quiet option)
func LogVerboseStderrQuiet(text string) {
	if os.Getenv("DOKKU_QUIET_OUTPUT") == "" {
		LogVerboseStderr(text)
	}
}

// LogVerboseQuietContainerLogs is the verbose log formatter for container logs
func LogVerboseQuietContainerLogs(containerID string) {
	LogVerboseQuietContainerLogsTail(containerID, 0, false)
}

// LogVerboseQuietContainerLogsTail is the verbose log formatter for container logs with tail mode enabled
func LogVerboseQuietContainerLogsTail(containerID string, lines int, tail bool) {
	args := []string{"container", "logs", containerID}
	if lines > 0 {
		args = append(args, "--tail", strconv.Itoa(lines))
	}
	if tail {
		args = append(args, "--follow")
	}

	var mu sync.Mutex
	result, err := CallExecCommand(ExecCommandInput{
		Command:            DockerBin(),
		Args:               args,
		DisableStdioBuffer: true,
		StdoutWriter: &writer{
			mu:     &mu,
			source: "stdout",
		},
		StderrWriter: &writer{
			mu:     &mu,
			source: "stderr",
		},
	})

	if err != nil {
		LogExclaim(fmt.Sprintf("Failed to fetch container logs: %s", containerID))
		return
	}

	if !tail && result.ExitCode != 0 {
		LogExclaim(fmt.Sprintf("Failed to fetch container logs: %s", containerID))
	}
}

// LogWarn is the warning log formatter
func LogWarn(text string) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(" !     %s", text))
}

// LogExclaim is the log exclaim formatter
func LogExclaim(text string) {
	fmt.Println(fmt.Sprintf(" !     %s", text))
}

// LogStderr is the stderr log formatter
func LogStderr(text string) {
	fmt.Fprintln(os.Stderr, text)
}

// LogDebug is the debug log formatter
func LogDebug(text string) {
	if os.Getenv("DOKKU_TRACE") == "1" {
		fmt.Fprintln(os.Stderr, fmt.Sprintf(" ?     %s", strings.TrimPrefix(text, " ?     ")))
	}
}
