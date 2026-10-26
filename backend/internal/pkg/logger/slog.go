package logger

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

const LevelFatal = slog.LevelError + 4

func init() {
	opts := tint.NewTextHandler(os.Stdout, &tint.Options{
		AddSource: false,
		Level:     slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				a.Value = slog.StringValue(a.Value.Time().Format(time.DateTime))
			case slog.LevelKey:
				// ERR+4 不好看，改名成 FTL 并给个亮红
				if lvl, ok := a.Value.Any().(slog.Level); ok && lvl >= LevelFatal {
					return tint.Attr(9, slog.String(a.Key, "FTL"))
				}
			}
			return a
		},
	})
	slog.SetDefault(slog.New(opts))
}

func Debug(msg string, args ...slog.Attr) {
	slog.LogAttrs(context.Background(), slog.LevelDebug, msg, args...)
}

func Info(msg string, args ...slog.Attr) {
	slog.LogAttrs(context.Background(), slog.LevelInfo, msg, args...)
}

func Warn(msg string, args ...slog.Attr) {
	slog.LogAttrs(context.Background(), slog.LevelWarn, msg, args...)
}

func Error(msg string, args ...slog.Attr) {
	slog.LogAttrs(context.Background(), slog.LevelError, msg, args...)
}

func Fatal(msg string, args ...slog.Attr) {
	slog.LogAttrs(context.Background(), LevelFatal, msg, args...)
	os.Exit(1)
}
