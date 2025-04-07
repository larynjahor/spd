package xslog

import (
	"io"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

const logLevel = slog.LevelInfo

func Auto() io.Closer {
	w, err := getWriter()
	if err != nil {
		log.Fatalln(err)
	}

	logger := slog.New(tint.NewHandler(w, &tint.Options{
		AddSource:   true,
		Level:       logLevel,
		ReplaceAttr: nil,
		TimeFormat:  time.Kitchen,
		NoColor:     false,
	}))

	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(logLevel)

	return w
}

func getWriter() (io.WriteCloser, error) {
	if !enable {
		return struct {
			io.Writer
			io.Closer
		}{
			io.Discard,
			io.NopCloser(nil),
		}, nil
	}

	return os.OpenFile("/tmp/spdlog", os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
}
