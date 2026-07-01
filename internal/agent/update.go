package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/tlmanz/klutch-agent/internal/store"
)

// updateManifest is the release descriptor the agent fetches. Each artifact is
// keyed by "<goos>/<goarch>" (e.g. "linux/amd64", "windows/amd64").
type updateManifest struct {
	Version   string                    `json:"version"`
	Artifacts map[string]updateArtifact `json:"artifacts"`
}

type updateArtifact struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// updateLoop checks for a newer release shortly after start and then on the
// configured interval. When AutoUpdate is on and a newer build exists it installs
// it and relaunches; otherwise it just records availability for the UI to offer.
func (a *Agent) updateLoop(ctx context.Context) {
	tick := func() {
		avail, err := a.CheckForUpdate(ctx)
		if err != nil {
			a.log.Printf("update check failed: %v", err)
			return
		}
		a.mu.Lock()
		auto := a.cfg.AutoUpdate
		a.mu.Unlock()
		if avail != "" && auto {
			a.log.Printf("auto-update: installing %s", avail)
			if err := a.ApplyUpdate(ctx); err != nil {
				a.log.Printf("auto-update failed: %v", err)
			}
		}
	}

	select {
	case <-ctx.Done():
		return
	case <-time.After(30 * time.Second):
	}
	tick()

	interval := a.cfg.UpdateInterval
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tick()
		}
	}
}

// CheckForUpdate fetches the manifest and records whether a newer build exists
// for this platform. It returns the newer version string, or "" if up to date.
// The result (and the check time) are persisted so the UI shows them after a
// restart.
func (a *Agent) CheckForUpdate(ctx context.Context) (string, error) {
	a.mu.Lock()
	url, cur := a.cfg.UpdateURL, a.cfg.Version
	a.mu.Unlock()
	if url == "" {
		return "", fmt.Errorf("self-update disabled (no update URL)")
	}

	m, err := fetchManifest(ctx, url)
	if err != nil {
		return "", err
	}
	now := time.Now()
	_ = a.store.SetSetting(store.KeyLastCheck, strconv.FormatInt(now.Unix(), 10))

	avail := ""
	if versionLess(cur, m.Version) {
		avail = m.Version
	}
	_ = a.store.SetSetting(store.KeyAvailableVersion, m.Version)
	a.mutate(func(s *State) {
		s.LastCheck = now
		s.AvailableVersion = avail
	})
	return avail, nil
}

// ApplyUpdate downloads the artifact for this platform, verifies its SHA-256,
// replaces the running binary, and relaunches the process. A "dev" build refuses
// to self-replace unless KLUTCH_AGENT_UPDATE_FORCE is set.
func (a *Agent) ApplyUpdate(ctx context.Context) error {
	a.mu.Lock()
	url, cur := a.cfg.UpdateURL, a.cfg.Version
	a.mu.Unlock()
	if url == "" {
		return fmt.Errorf("self-update disabled (no update URL)")
	}
	if cur == "dev" && os.Getenv("KLUTCH_AGENT_UPDATE_FORCE") == "" {
		return fmt.Errorf("refusing to replace a dev build (set KLUTCH_AGENT_UPDATE_FORCE=1 to override)")
	}

	m, err := fetchManifest(ctx, url)
	if err != nil {
		return err
	}
	key := runtime.GOOS + "/" + runtime.GOARCH
	art, ok := m.Artifacts[key]
	if !ok {
		return fmt.Errorf("manifest has no artifact for %s", key)
	}
	if !versionLess(cur, m.Version) {
		return fmt.Errorf("already up to date (%s)", cur)
	}
	if art.URL == "" {
		return fmt.Errorf("manifest artifact for %s has no url", key)
	}

	a.log.Printf("downloading update %s → %s (%s)", cur, m.Version, art.URL)
	bin, err := download(ctx, art.URL)
	if err != nil {
		return err
	}
	if art.SHA256 != "" {
		sum := sha256.Sum256(bin)
		if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, art.SHA256) {
			return fmt.Errorf("checksum mismatch: manifest %s, downloaded %s", art.SHA256, got)
		}
	}
	if err := replaceExecutable(bin); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	a.log.Printf("update installed; relaunching")
	if a.relaunch != nil {
		a.relaunch()
	}
	return nil
}

// fetchManifest downloads and decodes the update manifest.
func fetchManifest(ctx context.Context, url string) (updateManifest, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return updateManifest{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return updateManifest{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return updateManifest{}, fmt.Errorf("manifest fetch: HTTP %d", resp.StatusCode)
	}
	var m updateManifest
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m); err != nil {
		return updateManifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if m.Version == "" {
		return updateManifest{}, fmt.Errorf("manifest has no version")
	}
	return m, nil
}

// download fetches the binary bytes, following redirects (GitHub Releases assets
// redirect to a signed object URL), capped to a sane size.
func download(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}
	const maxBin = 200 << 20 // 200 MiB ceiling
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBin))
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("download: empty body")
	}
	return b, nil
}

// replaceExecutable writes the new binary next to the running one and swaps it in
// via swapFile (atomic rename on Unix, rename-aside on Windows). The temp file is
// created in the executable's own directory so the rename stays on one filesystem.
func replaceExecutable(newBin []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)

	tmp, err := os.CreateTemp(dir, ".klutch-agent-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(newBin)
	cerr := tmp.Close()
	if werr != nil {
		os.Remove(tmpName)
		return werr
	}
	if cerr != nil {
		os.Remove(tmpName)
		return cerr
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := swapFile(tmpName, exe); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// RemoveOldBinary deletes the "<exe>.old" sidecar left by a Windows self-update
// (a running .exe cannot be deleted, only renamed aside, so cleanup happens on
// the next start). A no-op on Unix, where the rename replaced the inode directly.
func RemoveOldBinary() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	_ = os.Remove(exe + ".old")
}

// Relaunch starts a fresh copy of the (now-replaced) binary with the same
// arguments and exits the current process. It is the default relaunch used after
// a self-update; the UI overrides it to quit the Fyne app cleanly first.
func Relaunch() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := cmd.Start(); err != nil {
		return
	}
	os.Exit(0)
}

// versionLess reports whether version a is strictly older than b, comparing
// dotted numeric components ("v1.2.0" < "v1.10.0"). A leading "v" is ignored and
// non-numeric components fall back to a string compare, so it degrades safely.
func versionLess(a, b string) bool {
	as := splitVersion(a)
	bs := splitVersion(b)
	for i := 0; i < len(as) || i < len(bs); i++ {
		av, bv := "0", "0" // a missing trailing component counts as 0 (1.2 == 1.2.0)
		if i < len(as) && as[i] != "" {
			av = as[i]
		}
		if i < len(bs) && bs[i] != "" {
			bv = bs[i]
		}
		an, aerr := strconv.Atoi(av)
		bn, berr := strconv.Atoi(bv)
		switch {
		case aerr == nil && berr == nil:
			if an != bn {
				return an < bn
			}
		case aerr != nil && berr == nil:
			return true // a non-numeric build (e.g. "dev") is older than a real release
		case aerr == nil && berr != nil:
			return false // a numeric release is newer than a non-numeric build
		default:
			if av != bv {
				return av < bv
			}
		}
	}
	return false
}

func splitVersion(v string) []string {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return nil
	}
	return strings.Split(v, ".")
}
