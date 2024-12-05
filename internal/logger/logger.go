package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Level represents the severity of a log message
type Level uint8

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

var levelPrefixes = map[Level]string{
	DebugLevel: "[DEBUG] ",
	InfoLevel:  "[INFO]  ",
	WarnLevel:  "[WARN]  ",
	ErrorLevel: "[ERROR] ",
	FatalLevel: "[FATAL] ",
}

// DefaultLogger implements the Logger interface
type DefaultLogger struct {
	logger *log.Logger
	file   *os.File
	level  Level
}

// New creates a new logger instance that writes to both a file and stdout
func New(appName string) (*DefaultLogger, error) {
	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02_15_0")
	logPath := filepath.Join(logsDir, fmt.Sprintf("%s_%s.log", appName, timestamp))
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	// Create multi-writer for both file and stdout
	multiWriter := io.MultiWriter(file, os.Stdout)

	return &DefaultLogger{
		logger: log.New(multiWriter, "", log.Ldate|log.Ltime|log.Lshortfile),
		file:   file,
		level:  InfoLevel, // Default to Info level
	}, nil
}

func (l *DefaultLogger) log(level Level, format string, v ...interface{}) {
	if level >= l.level {
		msg := fmt.Sprintf(format, v...)
		l.logger.Output(3, levelPrefixes[level]+msg)
	}
}

func (l *DefaultLogger) SetLevel(level Level) {
	l.level = level
}

func (l *DefaultLogger) Debug(format string, v ...interface{}) {
	l.log(DebugLevel, format, v...)
}

func (l *DefaultLogger) Info(format string, v ...interface{}) {
	l.log(InfoLevel, format, v...)
}

func (l *DefaultLogger) Warn(format string, v ...interface{}) {
	l.log(WarnLevel, format, v...)
}

func (l *DefaultLogger) Error(format string, v ...interface{}) {
	l.log(ErrorLevel, format, v...)
}

func (l *DefaultLogger) Fatal(format string, v ...interface{}) {
	l.log(FatalLevel, format, v...)
	os.Exit(1)
}

func (l *DefaultLogger) Close() error {
	return l.file.Close()
}
