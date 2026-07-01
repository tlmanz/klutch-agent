package main

// version is the agent's build version, stamped at link time:
//
//	go build -ldflags "-X main.version=v1.2.3"
//
// It defaults to "dev" for local builds. The self-updater (update.go) treats a
// "dev" build as always up-to-date so it never clobbers a developer's binary
// (override with KLUTCH_AGENT_UPDATE_FORCE=1 to test the update path).
var version = "dev"
