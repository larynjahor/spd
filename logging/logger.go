package logging

import (
	"io"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

func Auto() io.Closer {
	w, err := getWriter()
	if err != nil {
		log.Fatalln(err)
	}

	logLevel := slog.LevelDebug
	if !debug {
		logLevel = slog.LevelInfo
	}

	logger := slog.New(tint.NewHandler(w, &tint.Options{
		AddSource:   true,
		Level:       logLevel,
		ReplaceAttr: nil,
		TimeFormat:  time.Kitchen,
		NoColor:     !debug,
	}))

	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(logLevel)

	return w
}

func getWriter() (io.WriteCloser, error) {
	return struct {
		io.Writer
		io.Closer
	}{
		os.Stderr,
		io.NopCloser(nil),
	}, nil
}
