package logger

import (
	"log"
)

// Logger is an internal struct used to conditionally print messages 
// to stdout/stderr based on a verbose flag
type Logger struct {
	logger  *log.Logger
	verbose bool
}

// NewLogger creates a logger struct which conditionally logs 
// based on whether the verbose flag is set to true/false
func NewLogger(verboseOutput bool, logger *log.Logger) Logger {
	return Logger{logger, verboseOutput}
}

// Log is a wrapper around log.Println
// conditionally prints to output based on l.verbose value
func (l Logger) Log(msg string) {
	if l.verbose {
		l.logger.Println(msg)
	}
}

// Logf is a wrapper around log.Printf
// it also prints a new line
// conditionally prints to output based on l.verbose value
func (l Logger) Logf(format string, args ...interface{}) {
	if l.verbose {
		l.logger.Printf(format, args...)
		l.logger.Println()
	}
}
