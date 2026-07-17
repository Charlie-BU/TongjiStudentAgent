// Package log provides the application's lightweight logging facade.
package log

import (
	"context"
	stdlog "log"
	"os"
)

var logger = stdlog.New(os.Stderr, "", stdlog.LstdFlags|stdlog.Lmicroseconds|stdlog.LUTC)

// Infof writes an informational log message.
func Infof(format string, args ...any) {
	logger.Printf("INFO "+format, args...)
}

// Errorf writes an error log message.
func Errorf(format string, args ...any) {
	logger.Printf("ERROR "+format, args...)
}

// CtxInfo is the context-aware compatibility entry point. The standard logger
// does not inspect context values, so the context is intentionally unused.
func CtxInfo(_ context.Context, format string, args ...any) {
	Infof(format, args...)
}

// CtxError is the context-aware compatibility entry point for error messages.
func CtxError(_ context.Context, format string, args ...any) {
	Errorf(format, args...)
}

// Flush is kept for lifecycle compatibility. Standard library log writes are synchronous.
func Flush() {}
