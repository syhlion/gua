package main

import (
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// setupLogger builds the application slog.Logger from the environment:
//
//	LOG_OUTPUT  stdout (default) | file | both
//	LOG_FILE    path used when LOG_OUTPUT is file|both (default "gua.log")
//	LOG_LEVEL   debug | info (default) | warn | error
//	LOG_FORMAT  json (default) | text
//
// File output is rotated by lumberjack:
//
//	LOG_ROTATE_MAX_SIZE_MB   rotate after this many MB   (default 100)
//	LOG_ROTATE_MAX_BACKUPS   keep at most N rotated files (default 7)
//	LOG_ROTATE_MAX_AGE_DAYS  delete rotated files older than N days (default 30)
//	LOG_ROTATE_COMPRESS      gzip rotated files (default false)
func setupLogger() *slog.Logger {
	out := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_OUTPUT")))
	if out == "" {
		out = "stdout"
	}
	path := strings.TrimSpace(os.Getenv("LOG_FILE"))
	if path == "" {
		path = "gua.log"
	}

	wantStdout := out == "stdout" || out == "both"
	wantFile := out == "file" || out == "both"

	var writers []io.Writer
	if wantStdout {
		writers = append(writers, os.Stdout)
	}
	if wantFile {
		// lumberjack creates the file (and dir) on first write and rotates it.
		writers = append(writers, &lumberjack.Logger{
			Filename:   path,
			MaxSize:    envInt("LOG_ROTATE_MAX_SIZE_MB", 100),
			MaxBackups: envInt("LOG_ROTATE_MAX_BACKUPS", 7),
			MaxAge:     envInt("LOG_ROTATE_MAX_AGE_DAYS", 30),
			Compress:   envBool("LOG_ROTATE_COMPRESS", false),
		})
	}
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}
	w := io.MultiWriter(writers...)

	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}

	var h slog.Handler
	if strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT"))) == "text" {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	return slog.New(h)
}

// logFatal logs at error level and exits (slog has no Fatal).
func logFatal(l *slog.Logger, msg string, args ...any) {
	l.Error(msg, args...)
	os.Exit(1)
}
