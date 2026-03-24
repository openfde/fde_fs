package logger

import (
	"os"
	"strconv"
	"syscall"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/warlice/lumberjack"
)

// StandardLogger struct for sentry
type StandardLogger struct {
	*logrus.Logger
}

var (
	Logger = NewLogger() //Logger New logger by loggerSentry and loggerLine
)

var logFile = "/var/log/fde.log"

func Init() *StandardLogger {
	var baseLogger = logrus.New()
	var standard = &StandardLogger{baseLogger}

	levelStr := strings.TrimSpace(os.Getenv("FDE_LOG_LEVEL"))
	if levelStr == "" {
		standard.SetLevel(logrus.ErrorLevel)
	} else {
		// try parse as level name first (e.g., "info", "warn")
		if lvl, err := logrus.ParseLevel(strings.ToLower(levelStr)); err == nil {
			standard.SetLevel(lvl)
		} else if n, err := strconv.Atoi(levelStr); err == nil {
			// fall back to numeric level
			standard.SetLevel(logrus.Level(n))
		} else {
			// invalid value, use default
			standard.SetLevel(logrus.ErrorLevel)
		}
	}

	standard.Formatter = &logrus.JSONFormatter{}
	return standard
}

var LumberLogger *lumberjack.Logger

func Rotate() {
	if LumberLogger != nil {
		err := LumberLogger.Rotate()
		if err != nil {
			Logger.WithError(err).Error("log_rotate_failed")
		}
	}
}

// NewLogger New logger by  loggerLine
func NewLogger() *StandardLogger {
	standard := Init()
	syscall.Umask(0)
	logName := logFile
	if len(logName) != 0 {
		LumberLogger = &lumberjack.Logger{
			Filename:   logName,
			MaxSize:    10, // megabytes
			MaxBackups: 1,
			MaxAge:     30,   //days
			Compress:   true, // disabled by default
		}
		standard.SetOutput(LumberLogger)
	}
	standard.loggerLine()
	return standard
}

func UpdateLoggerLevel(Level uint32) {
	var level = logrus.Level(Level)
	if level > logrus.TraceLevel || level == 0 {
		level = logrus.TraceLevel
	}
	Logger.SetLevel(level)
}

// loggerLine for print log with line
func (logger *StandardLogger) loggerLine() {
	hookWithLine := NewContextLine()
	logger.Hooks.Add(hookWithLine)
}

func Info(from string, customize interface{}) {
	buildLogEntry(from, customize).Info()
}

func Error(from string, customize interface{}, error error) {
	buildLogEntry(from, customize, error).Error()
}

func Warn(from string, customize interface{}, errors ...error) {
	buildLogEntry(from, customize, errors...).Warn()
	return
}

func buildLogEntry(from string, customize interface{}, errors ...error) *logrus.Entry {
	fields := logrus.Fields{
		"from":   from,
		"source": customize,
	}
	if len(errors) > 0 {
		errLogs := make([]logrus.Fields, 0)
		for _, err := range errors {
			if err != nil {
				errLogs = append(errLogs, generateErrorFields(err))
			}
		}
		fields["errors"] = errLogs
	}
	return Logger.WithFields(fields)
}

func generateErrorFields(err error) logrus.Fields {
	return logrus.Fields{
		"err": err.Error(),
	}
}
