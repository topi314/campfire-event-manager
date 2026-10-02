package main

//go:generate sqlc generate

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/topi314/campfire-event-manager/server"

	_ "time/tzdata" // embed IANA zones so LoadLocation works without host tzdata
)

func main() {
	cfgPath := flag.String("config", "config.toml", "path to config file")
	flag.Parse()

	cfg, err := server.LoadConfig(*cfgPath)
	if err != nil {
		slog.Error("Error while loading config", slog.Any("err", err))
		os.Exit(1)
	}

	setupLogger(cfg.Log)

	version := "unknown"
	goVersion := "unknown"
	if info, ok := debug.ReadBuildInfo(); ok {
		version = info.Main.Version
		goVersion = info.GoVersion
	}

	slog.Info("Starting server...", slog.String("version", version), slog.String("go_version", goVersion))
	slog.Info("Config loaded", slog.Any("config", cfg.String()))

	srv, err := server.New(cfg)
	if err != nil {
		slog.Error("Error while creating server", slog.Any("err", err))
		os.Exit(1)
	}

	go srv.Start()
	defer srv.Stop()

	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGTERM, syscall.SIGINT)
	<-s
}

func setupLogger(cfg server.LogConfig) {
	opts := &slog.HandlerOptions{
		AddSource: cfg.AddSource,
		Level:     cfg.Level,
	}
	var handler slog.Handler
	switch cfg.Format {
	case server.LogFormatJSON:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case server.LogFormatText:
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		slog.Error("Unknown log format", slog.String("format", string(cfg.Format)))
		os.Exit(1)
	}
	slog.SetDefault(slog.New(handler))
}
