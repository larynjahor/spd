package xslog

import (
	"io"
	"log"
	"log/slog"
	"os"
)

func Auto() io.Closer {
	w, err := getWriter()
	if err != nil {
		log.Fatalln(err)
	}

	logger := slog.New(slog.NewTextHandler(w, nil))

	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(slog.LevelDebug)

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

	return os.CreateTemp("/tmp", "spdlog*")
}
