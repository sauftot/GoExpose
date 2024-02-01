package logger

import (
	"log/slog"
	"os"
)

const (
	DEBUG = uint8(1)
	INFO  = uint8(2)
)

type Logger struct {
	consoleLogger *slog.Logger
	fileLogger    *slog.Logger
	loglevel      uint8
}

func NewLogger(logFileName string) (*Logger, error) {
	file, err := os.OpenFile(logFileName+".log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return &Logger{
		consoleLogger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
		fileLogger:    slog.New(slog.NewTextHandler(file, nil)),
	}, nil
}

func (l *Logger) SetLogLevel(level uint8) {
	l.loglevel = level
}

func (l *Logger) Log(msg string) {
	if l.loglevel == DEBUG {
		l.consoleLogger.Debug(msg)
		l.fileLogger.Debug(msg)
	} else if l.loglevel == INFO {
		l.fileLogger.Info(msg)
	}
}

func (l *Logger) Error(msg string, err error) {
	l.consoleLogger.Error(msg, err)
	l.fileLogger.Error(msg, err)
}
