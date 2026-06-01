// Command speedtickr shows live network upload/download speed in the system tray.
package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/codeseasy/speedtickr/internal/config"
	"github.com/codeseasy/speedtickr/internal/format"
	"github.com/codeseasy/speedtickr/internal/singleton"
	"github.com/codeseasy/speedtickr/internal/tray"
)

// version is the release version, overridden at build time with -ldflags "-X main.version=...".
var version = "1.0.0"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	unitFlag := flag.String("unit", "", "display unit: bps, kbps, mbps, gbps, or tbps (overrides config)")
	intervalFlag := flag.Duration("interval", 0, "update interval, e.g. 1s (overrides config)")
	debug := flag.Bool("debug", false, "write debug logs to <temp>/speedtickr.log")
	flag.Parse()

	if *showVersion {
		fmt.Println("speedtickr", version)
		return
	}

	setupLogging(*debug)

	// Only one meter per user session: a second launch detects the first and exits.
	release, ok, err := singleton.Acquire()
	if err != nil {
		slog.Warn("single-instance check failed; starting anyway", "err", err)
	} else if !ok {
		slog.Info("SpeedTickr is already running; exiting this instance")
		return
	}
	if release != nil {
		defer release()
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Warn("using default config", "err", err)
	}

	if *unitFlag != "" {
		if u, ok := format.ParseUnit(*unitFlag); ok {
			cfg.Unit = u
		} else {
			slog.Warn("ignoring invalid -unit", "value", *unitFlag)
		}
	}
	if *intervalFlag > 0 {
		cfg.Interval = config.Duration(*intervalFlag)
	}
	cfg.Normalize() // re-clamp after flag overrides (Load already normalized the file)

	if err := tray.Run(cfg); err != nil {
		slog.Error("speedtickr exited with error", "err", err)
		os.Exit(1)
	}
}

// setupLogging sends logs to stderr at info level, or to <temp>/speedtickr.log at
// debug level when -debug is set (useful since the GUI build has no console).
func setupLogging(debug bool) {
	level := slog.LevelInfo
	var w io.Writer = os.Stderr
	if debug {
		level = slog.LevelDebug
		if f, err := os.OpenFile(filepath.Join(os.TempDir(), "speedtickr.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
			w = f
		}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})))
}
