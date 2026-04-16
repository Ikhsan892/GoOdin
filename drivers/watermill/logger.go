package watermill

import (
	"log/slog"

	"github.com/ThreeDotsLabs/watermill"
)

// watermillSlogLogger adapts slog.Logger to watermill.LoggerAdapter.
type watermillSlogLogger struct {
	log *slog.Logger
}

func newWatermillLogger(log *slog.Logger) watermill.LoggerAdapter {
	return &watermillSlogLogger{log: log}
}

func (l *watermillSlogLogger) Error(msg string, err error, fields watermill.LogFields) {
	args := logFieldsToArgs(fields)
	if err != nil {
		args = append(args, slog.String("error", err.Error()))
	}
	l.log.Error("[watermill] "+msg, args...)
}

func (l *watermillSlogLogger) Info(msg string, fields watermill.LogFields) {
	l.log.Info("[watermill] "+msg, logFieldsToArgs(fields)...)
}

func (l *watermillSlogLogger) Debug(msg string, fields watermill.LogFields) {
	l.log.Debug("[watermill] "+msg, logFieldsToArgs(fields)...)
}

func (l *watermillSlogLogger) Trace(msg string, fields watermill.LogFields) {
	l.log.Debug("[watermill:trace] "+msg, logFieldsToArgs(fields)...)
}

func (l *watermillSlogLogger) With(fields watermill.LogFields) watermill.LoggerAdapter {
	return &watermillSlogLogger{log: l.log.With(logFieldsToArgs(fields)...)}
}

func logFieldsToArgs(fields watermill.LogFields) []any {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, slog.Any(k, v))
	}
	return args
}
