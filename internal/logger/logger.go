package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel string

const (
	DebugLevel LogLevel = "DEBUG"
	InfoLevel  LogLevel = "INFO"
	WarnLevel  LogLevel = "WARN"
	ErrorLevel LogLevel = "ERROR"
)

// Fields type for structured logging
type Fields map[string]interface{}

// Logger represents a structured logger
type Logger struct {
	logger *log.Logger
	fields Fields
	level  LogLevel
}

// New creates a new Logger instance
func New() *Logger {
	return &Logger{
		logger: log.New(os.Stdout, "", 0), // No prefix, we'll format everything ourselves
		fields: make(Fields),
		level:  InfoLevel, // Default level
	}
}

// WithLevel sets the log level for the logger
func (l *Logger) WithLevel(level LogLevel) *Logger {
	l.level = level
	return l
}

// WithField adds a single field to the logger
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l.WithFields(Fields{key: value})
}

// WithFields adds multiple fields to the logger
func (l *Logger) WithFields(fields Fields) *Logger {
	newFields := make(Fields, len(l.fields)+len(fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &Logger{
		logger: l.logger,
		fields: newFields,
		level:  l.level,
	}
}

// WithError adds an error field to the logger
func (l *Logger) WithError(err error) *Logger {
	return l.WithField("error", err.Error())
}

// WithRequestID adds a request ID to the logger
func (l *Logger) WithRequestID(requestID string) *Logger {
	return l.WithField("request_id", requestID)
}

// log is the internal logging function that handles the actual writing
func (l *Logger) log(level LogLevel, msg string, fields Fields) {
	// Skip if log level is too low
	if l.shouldSkip(level) {
		return
	}

	entry := l.prepareEntry(level, msg, fields)
	l.write(entry)
}

// shouldSkip returns true if the log level is below the configured level
func (l *Logger) shouldSkip(level LogLevel) bool {
	return levelToInt(level) < levelToInt(l.level)
}

// prepareEntry creates a log entry with all necessary fields
func (l *Logger) prepareEntry(level LogLevel, msg string, fields Fields) map[string]interface{} {
	entry := make(Fields, len(l.fields)+len(fields)+3)

	// Add timestamp
	entry["timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)

	// Add log level
	entry["level"] = string(level)

	// Add message
	entry["message"] = msg

	// Add caller info (file and line number)
	if _, file, line, ok := runtime.Caller(3); ok {
		short := file
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				break
			}
		}
		entry["caller"] = fmt.Sprintf("%s:%d", short, line)
	}

	// Add logger fields
	for k, v := range l.fields {
		entry[k] = v
	}

	// Add log-specific fields
	for k, v := range fields {
		entry[k] = v
	}

	return entry
}

// write outputs the log entry as JSON
func (l *Logger) write(entry map[string]interface{}) {
	jsonData, err := json.Marshal(entry)
	if err != nil {
		l.logger.Printf("{\"level\":\"ERROR\",\"message\":\"Failed to marshal log entry: %v\"}", err)
		return
	}

	l.logger.Println(string(jsonData))
}

// Debug logs a message at Debug level
func (l *Logger) Debug(msg string) {
	l.log(DebugLevel, msg, nil)
}

// Info logs a message at Info level
func (l *Logger) Info(msg string) {
	l.log(InfoLevel, msg, nil)
}

// Warn logs a message at Warn level
func (l *Logger) Warn(msg string) {
	l.log(WarnLevel, msg, nil)
}

// Error logs a message at Error level
func (l *Logger) Error(msg string) {
	l.log(ErrorLevel, msg, nil)
}

// Debugf logs a formatted message at Debug level
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DebugLevel, fmt.Sprintf(format, args...), nil)
}

// Infof logs a formatted message at Info level
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(InfoLevel, fmt.Sprintf(format, args...), nil)
}

// Warnf logs a formatted message at Warn level
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WarnLevel, fmt.Sprintf(format, args...), nil)
}

// Errorf logs a formatted message at Error level
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ErrorLevel, fmt.Sprintf(format, args...), nil)
}

// DebugWithFields logs a message with fields at Debug level
func (l *Logger) DebugWithFields(msg string, fields Fields) {
	l.log(DebugLevel, msg, fields)
}

// InfoWithFields logs a message with fields at Info level
func (l *Logger) InfoWithFields(msg string, fields Fields) {
	l.log(InfoLevel, msg, fields)
}

// WarnWithFields logs a message with fields at Warn level
func (l *Logger) WarnWithFields(msg string, fields Fields) {
	l.log(WarnLevel, msg, fields)
}

// ErrorWithFields logs a message with fields at Error level
func (l *Logger) ErrorWithFields(msg string, fields Fields) {
	l.log(ErrorLevel, msg, fields)
}

// levelToInt converts a LogLevel to an integer for comparison
func levelToInt(level LogLevel) int {
	switch strings.ToUpper(string(level)) {
	case "DEBUG":
		return 0
	case "INFO":
		return 1
	case "WARN":
		return 2
	case "ERROR":
		return 3
	default:
		return 1 // Default to INFO
	}
}

// Default logger instance
var defaultLogger = New()

// SetLevel sets the log level for the default logger
func SetLevel(level LogLevel) {
	defaultLogger = defaultLogger.WithLevel(level)
}

// WithField adds a field to the default logger
func WithField(key string, value interface{}) *Logger {
	return defaultLogger.WithField(key, value)
}

// WithFields adds fields to the default logger
func WithFields(fields Fields) *Logger {
	return defaultLogger.WithFields(fields)
}

// WithError adds an error to the default logger
func WithError(err error) *Logger {
	return defaultLogger.WithError(err)
}

// WithRequestID adds a request ID to the default logger
func WithRequestID(requestID string) *Logger {
	return defaultLogger.WithRequestID(requestID)
}

// Debug logs a message at Debug level using the default logger
func Debug(msg string) {
	defaultLogger.Debug(msg)
}

// Info logs a message at Info level using the default logger
func Info(msg string) {
	defaultLogger.Info(msg)
}

// Warn logs a message at Warn level using the default logger
func Warn(msg string) {
	defaultLogger.Warn(msg)
}

// Error logs a message at Error level using the default logger
func Error(msg string) {
	defaultLogger.Error(msg)
}

// Debugf logs a formatted message at Debug level using the default logger
func Debugf(format string, args ...interface{}) {
	defaultLogger.Debugf(format, args...)
}

// Infof logs a formatted message at Info level using the default logger
func Infof(format string, args ...interface{}) {
	defaultLogger.Infof(format, args...)
}

// Warnf logs a formatted message at Warn level using the default logger
func Warnf(format string, args ...interface{}) {
	defaultLogger.Warnf(format, args...)
}

// Errorf logs a formatted message at Error level using the default logger
func Errorf(format string, args ...interface{}) {
	defaultLogger.Errorf(format, args...)
}

// DebugWithFields logs a message with fields at Debug level using the default logger
func DebugWithFields(msg string, fields Fields) {
	defaultLogger.DebugWithFields(msg, fields)
}

// InfoWithFields logs a message with fields at Info level using the default logger
func InfoWithFields(msg string, fields Fields) {
	defaultLogger.InfoWithFields(msg, fields)
}

// WarnWithFields logs a message with fields at Warn level using the default logger
func WarnWithFields(msg string, fields Fields) {
	defaultLogger.WarnWithFields(msg, fields)
}

// ErrorWithFields logs a message with fields at Error level using the default logger
func ErrorWithFields(msg string, fields Fields) {
	defaultLogger.ErrorWithFields(msg, fields)
}

// RequestLogger creates a new logger with request context
func RequestLogger(r *http.Request) *Logger {
	if r == nil {
		return defaultLogger
	}
	return defaultLogger.WithFields(Fields{
		"method":      r.Method,
		"path":        r.URL.Path,
		"remote_addr": r.RemoteAddr,
	})
}
