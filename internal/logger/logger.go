package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level defines log severity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger is a structured concurrency-safe logger.
type Logger struct {
	mu        sync.Mutex
	out       io.Writer
	minLevel  Level
	jsonMode  bool
	noColor   bool
	prefix    string
}

var defaultLogger = New(os.Stderr, LevelInfo, false, false)

// New creates a new Logger instance.
func New(out io.Writer, minLevel Level, jsonMode bool, noColor bool) *Logger {
	return &Logger{
		out:      out,
		minLevel: minLevel,
		jsonMode: jsonMode,
		noColor:  noColor,
	}
}

// SetDefault sets global default logger.
func SetDefault(l *Logger) {
	defaultLogger = l
}

// GetDefault returns the global default logger.
func GetDefault() *Logger {
	return defaultLogger
}

// SetLevel updates log level.
func (l *Logger) SetLevel(lvl Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.minLevel = lvl
}

// SetJSONMode toggles JSON logging.
func (l *Logger) SetJSONMode(enable bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.jsonMode = enable
}

func (l *Logger) log(lvl Level, msg string, keyvals ...any) {
	if lvl < l.minLevel {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	if l.jsonMode {
		payload := map[string]any{
			"time":  now.Format(time.RFC3339Nano),
			"level": lvl.String(),
			"msg":   msg,
		}
		for i := 0; i < len(keyvals); i += 2 {
			if i+1 < len(keyvals) {
				key := fmt.Sprintf("%v", keyvals[i])
				payload[key] = keyvals[i+1]
			}
		}
		data, _ := json.Marshal(payload)
		fmt.Fprintln(l.out, string(data))
		return
	}

	timeStr := now.Format("15:04:05.000")
	var lvlStr string
	if l.noColor {
		lvlStr = fmt.Sprintf("[%s]", lvl.String())
	} else {
		switch lvl {
		case LevelDebug:
			lvlStr = fmt.Sprintf("\033[36m[%s]\033[0m", lvl.String())
		case LevelInfo:
			lvlStr = fmt.Sprintf("\033[32m[%s]\033[0m", lvl.String())
		case LevelWarn:
			lvlStr = fmt.Sprintf("\033[33m[%s]\033[0m", lvl.String())
		case LevelError:
			lvlStr = fmt.Sprintf("\033[31m[%s]\033[0m", lvl.String())
		}
	}

	fields := ""
	if len(keyvals) > 0 {
		var fieldParts []string
		for i := 0; i < len(keyvals); i += 2 {
			if i+1 < len(keyvals) {
				fieldParts = append(fieldParts, fmt.Sprintf("%v=%v", keyvals[i], keyvals[i+1]))
			}
		}
		fields = " " + fmt.Sprintf("%v", fieldParts)
	}

	fmt.Fprintf(l.out, "%s %s %s%s\n", timeStr, lvlStr, msg, fields)
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, keyvals ...any) {
	l.log(LevelDebug, msg, keyvals...)
}

// Info logs at info level.
func (l *Logger) Info(msg string, keyvals ...any) {
	l.log(LevelInfo, msg, keyvals...)
}

// Warn logs at warning level.
func (l *Logger) Warn(msg string, keyvals ...any) {
	l.log(LevelWarn, msg, keyvals...)
}

// Error logs at error level.
func (l *Logger) Error(msg string, keyvals ...any) {
	l.log(LevelError, msg, keyvals...)
}

// Package-level delegations
func Debug(msg string, keyvals ...any) { defaultLogger.Debug(msg, keyvals...) }
func Info(msg string, keyvals ...any)  { defaultLogger.Info(msg, keyvals...) }
func Warn(msg string, keyvals ...any)  { defaultLogger.Warn(msg, keyvals...) }
func Error(msg string, keyvals ...any) { defaultLogger.Error(msg, keyvals...) }

func Debugf(format string, args ...any) { defaultLogger.Debug(fmt.Sprintf(format, args...)) }
func Infof(format string, args ...any)  { defaultLogger.Info(fmt.Sprintf(format, args...)) }
func Warnf(format string, args ...any)  { defaultLogger.Warn(fmt.Sprintf(format, args...)) }
func Errorf(format string, args ...any) { defaultLogger.Error(fmt.Sprintf(format, args...)) }
