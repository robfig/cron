package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/robfig/cron/v3"
)

type cronLogger struct{}

func (t *cronLogger) Printf(format string, args ...interface{}) {
	slog.Info(args[0].(string), append([]any{
		"logGroup", "cron",
	}, args[1:]...)...)
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	var err error
	// main
	spec := "0 0 0 * * *"
	if len(os.Args) == 2 && os.Args[1] != "" {
		spec = os.Args[1]
	}

	slog.Info("start", "spec", spec, "args", os.Args)

	logger := &cronLogger{}
	c := cron.New(cron.WithSeconds(), cron.WithLogger(cron.VerbosePrintfLogger(logger)))

	c.Start()

	_, err = c.AddFunc(spec, func() {
		now := time.Now()
		slog.Info("job triggered", "spec", spec, "now", now)
	})
	if err != nil {
		panic(err)
	}

	select {}
}
