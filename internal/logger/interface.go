package logger

// Logger defines the logging interface used across packages.
// Any type implementing these methods can be used as a logger
// throughout the application.
type Logger interface {
	Debug(format string, v ...interface{})
	Info(format string, v ...interface{})
	Warn(format string, v ...interface{})
	Error(format string, v ...interface{})
	Fatal(format string, v ...interface{})
} 