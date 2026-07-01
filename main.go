// Command klutch-agent is the Klutch print agent: a desktop app that runs on a
// shop PC, dials the backend over WSS, advertises its printers, prints the jobs
// pushed to it (PDF or ESC/POS raster), and reports status. It renders nothing
// itself (the backend does); it only transports bytes to the OS spooler.
//
// It ships a Fyne UI to view printers, browse the print-job history, change
// settings, and check/install updates, backed by a local SQLite store. It keeps
// running in the system tray and self-updates from its release channel. Pass
// -headless to run with no UI (for a server or unattended box).
//
// It shares ONLY the wire DTOs with the backend (the wire package), never domain
// types, which is why it lives in its own repository (github.com/tlmanz/klutch-agent).
//
// Config (flags override env; the in-app Settings persist to the local store and
// win on the next launch):
//
//	KLUTCH_AGENT_SERVER           backend base URL (e.g. https://api.example.com)
//	KLUTCH_AGENT_PAIRING_CODE     one-time pairing code (redeemed on first run)
//	KLUTCH_AGENT_TOKEN            pre-issued device token (provisioning; skips pairing)
//	KLUTCH_AGENT_DATA_DIR         where the SQLite DB + spool live
//	KLUTCH_AGENT_FALLBACK_PRINTER name to advertise if OS enumeration finds none
//	KLUTCH_AGENT_REFRESH          printer re-enumeration interval (default 30s)
//	KLUTCH_AGENT_UPDATE_URL       self-update manifest URL ("" disables self-update)
//	KLUTCH_AGENT_UPDATE_INTERVAL  self-update check interval (default 6h)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tlmanz/klutch-agent/internal/agent"
	"github.com/tlmanz/klutch-agent/internal/store"
	"github.com/tlmanz/klutch-agent/internal/ui"
)

// defaultUpdateURL points at the manifest published with every GitHub Release of
// this (public) repo. "latest/download" always resolves to the newest release's
// asset, so the agent needs no version number to find the current manifest.
const defaultUpdateURL = "https://github.com/tlmanz/klutch-agent/releases/latest/download/manifest.json"

func main() {
	log.SetPrefix("klutch-agent: ")
	log.SetFlags(log.LstdFlags | log.LUTC)

	var (
		headless    = flag.Bool("headless", false, "run without the GUI (unattended)")
		showVersion = flag.Bool("version", false, "print version and exit")
		tray        = flag.Bool("tray", false, "start minimised to the system tray")
		server      = flag.String("server", env("KLUTCH_AGENT_SERVER", ""), "backend base URL")
		pairingCode = flag.String("pairing-code", os.Getenv("KLUTCH_AGENT_PAIRING_CODE"), "one-time pairing code")
		token       = flag.String("token", os.Getenv("KLUTCH_AGENT_TOKEN"), "pre-issued device token (provisioning; skips pairing)")
		updateURL   = flag.String("update-url", env("KLUTCH_AGENT_UPDATE_URL", defaultUpdateURL), "self-update manifest URL")
		noUpdate    = flag.Bool("no-update", false, "disable self-update")
		dataDirFlag = flag.String("data", env("KLUTCH_AGENT_DATA_DIR", dataDir()), "data directory (SQLite DB + spool)")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	// A prior Windows self-update leaves a "<exe>.old" sidecar; clean it up.
	agent.RemoveOldBinary()

	if err := os.MkdirAll(*dataDirFlag, 0o755); err != nil {
		log.Fatalf("data dir: %v", err)
	}
	st, err := store.Open(filepath.Join(*dataDirFlag, "agent.db"))
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	if *noUpdate {
		*updateURL = ""
	}
	cfg := agent.Config{
		Server:          *server,
		FallbackPrinter: env("KLUTCH_AGENT_FALLBACK_PRINTER", "Default Printer"),
		SpoolDir:        filepath.Join(*dataDirFlag, "spool"),
		Refresh:         envDuration("KLUTCH_AGENT_REFRESH", 30*time.Second),
		UpdateURL:       *updateURL,
		UpdateInterval:  envDuration("KLUTCH_AGENT_UPDATE_INTERVAL", 6*time.Hour),
		Version:         version,
	}
	log.Printf("starting version %s (data %s)", version, *dataDirFlag)

	ag := agent.New(cfg, st, log.Default())

	// Apply a server override from the flag/env, and enroll if credentials were
	// supplied and we are not enrolled yet.
	if *server != "" && *server != ag.Snapshot().Server {
		_ = ag.SetServer(*server)
	}

	// A pre-issued device token (provisioning) wins over pairing: install it
	// directly and skip the pairing-code redeem.
	if *token != "" && !ag.Snapshot().Enrolled {
		if err := ag.SetToken(*token); err != nil {
			log.Printf("install token: %v", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *pairingCode != "" && !ag.Snapshot().Enrolled {
		go func() {
			if err := ag.Enroll(ctx, *pairingCode); err != nil {
				log.Printf("enrollment failed: %v", err)
			}
		}()
	}

	if *headless {
		ag.Run(ctx) // blocks until signalled
		return
	}

	// GUI: run the agent in the background and show the window (or tray).
	go ag.Run(ctx)
	view := ui.New(ctx, ag)
	view.SetStartHidden(*tray)
	view.Run() // blocks on the Fyne event loop
}

// dataDir returns the default per-user data directory for the DB + spool.
func dataDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		if home, herr := os.UserHomeDir(); herr == nil {
			base = home
		} else {
			base = "."
		}
	}
	return filepath.Join(base, "klutch-agent")
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
