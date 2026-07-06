package main

import (
	"context"
	"embed"

	"github.com/tlmanz/klutch-agent/internal/agent"
	"github.com/tlmanz/klutch-agent/internal/desktopapp"
)

// assets embeds the built React frontend. The embed lives in package main
// because go:embed cannot reach across parent directories, so it is threaded
// into the desktopapp package which owns the Wails bootstrap.
//
//go:embed all:frontend/dist
var assets embed.FS

// runDesktop launches the Wails GUI and blocks until it exits.
func runDesktop(ctx context.Context, ag *agent.Agent, startHidden bool, registerActivate func(func())) error {
	return desktopapp.Run(ctx, ag, assets, startHidden, registerActivate)
}
