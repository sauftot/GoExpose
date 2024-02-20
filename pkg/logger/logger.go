package logger

import (
	"log/slog"
	"os"
)

/*
	Logger package provides a simple logging interface for applications using the kinda new slog library.
	It's a simple wrapper for a console and a file logger.
	When the level is DEBUG, everything will be printed to the console and the logfile. When the level is INFO, only
	the logfile will be written to, unless Error(string, err) is called. This is not a proper loglevel implementation,
	but it has its use-cases.
*/

const (
	DEBUG = uint8(1)
	INFO  = uint8(2)
)

type Logger struct {
	consoleLogger *slog.Logger
	fileLogger    *slog.Logger
	loglevel      uint8
}

// NewLogger function creates a new Logger instance.
// It takes a logFileName as input and opens a file with that name for logging.
// It returns a pointer to the Logger instance and an error if any occurred during the file opening.
func NewLogger(logFileName string) (*Logger, error) {
	file, err := os.OpenFile(logFileName+".log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return nil, err
	}
	return &Logger{
		consoleLogger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
		fileLogger:    slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}, nil
}

// SetLogLevel method sets the log level of the Logger instance.
// It takes a level as input and sets the loglevel of the Logger to that level.
func (l *Logger) SetLogLevel(level uint8) {
	l.loglevel = level
}

// Log method logs a message with the Logger instance.
// It takes a message as input and logs it to the console and/or file depending on the loglevel of the Logger.
func (l *Logger) Log(msg string, args ...any) {
	if l.loglevel == DEBUG {
		l.consoleLogger.Debug(msg, args...)
		l.fileLogger.Debug(msg, args...)
	} else if l.loglevel == INFO {
		l.fileLogger.Info(msg, args...)
	}
}

func (l *Logger) Debug(msg string, args ...any) {
	if l.loglevel == DEBUG {
		l.consoleLogger.Debug(msg, args...)
		l.fileLogger.Debug(msg, args...)
	}
}

func (l *Logger) Info(msg string, args ...any) {
	l.fileLogger.Info(msg, args...)
}

// Error method logs an error message with the Logger instance.
// It takes a message and an error as input and logs them to the console and file.
func (l *Logger) Error(msg string, args ...any) {
	l.consoleLogger.Error(msg, args...)
	l.fileLogger.Error(msg, args...)
}
