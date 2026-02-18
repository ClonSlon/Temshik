package logging

import (
	"io"
	"log/slog"
	"os"
)

type CloseFunc func() error

func Init(logPath string, alsoStdout bool) (*slog.Logger, CloseFunc, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}

	var out io.Writer = logFile
	if alsoStdout {
		out = io.MultiWriter(logFile, os.Stdout)
	}

	handler := slog.NewTextHandler(out, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger, logFile.Close, nil
}

