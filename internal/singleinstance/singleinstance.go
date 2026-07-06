// Package singleinstance enforces that only one agent runs per user data
// directory. A second launch - e.g. re-opening the app from the launcher while
// the first instance sits in the tray - must not start a rival agent (two agents
// would fight over the backend socket), it should raise the running one instead.
//
// Mechanism (stdlib only, identical on every OS): the first instance listens on
// a loopback TCP port and records "<port>\n<token>" in <dir>/instance.lock. A
// later instance reads the file, connects, and presents the token; the primary
// verifies it, replies OK, and runs its activate callback (show the window). The
// newcomer then exits. A stale lock (crash, or nothing listening) fails the
// handshake, so the newcomer simply becomes the primary and overwrites it.
//
// Loopback binding needs no firewall grant; the token (readable only by the
// owning user, 0600) stops other local users from triggering activation.
package singleinstance

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const lockFileName = "instance.lock"

// Instance is a handle to this process's participation in the single-instance
// protocol. When Primary is true the process owns the lock and should run
// normally; otherwise another instance was already running and this process
// should exit.
type Instance struct {
	primary  bool
	ln       net.Listener
	lockPath string
	token    string

	mu       sync.Mutex
	activate func()
}

// Acquire attempts to become the single primary instance for dir. If another
// live instance is found it is signalled to activate and the returned Instance
// reports Primary() == false. dir must already exist.
func Acquire(dir string) (*Instance, error) {
	lockPath := filepath.Join(dir, lockFileName)

	// If a live primary answers the handshake, signal it and stand down.
	if signalExisting(lockPath) {
		return &Instance{primary: false, lockPath: lockPath}, nil
	}

	// Become the primary: bind a loopback port and publish it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	token, err := randToken()
	if err != nil {
		ln.Close()
		return nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := writeLock(lockPath, port, token); err != nil {
		ln.Close()
		return nil, err
	}

	inst := &Instance{primary: true, ln: ln, lockPath: lockPath, token: token}
	go inst.serve()
	return inst, nil
}

// Primary reports whether this process owns the single-instance lock.
func (i *Instance) Primary() bool { return i != nil && i.primary }

// SetActivate registers the callback run when a later instance asks the primary
// to come to the foreground. Safe to call at any time; a nil callback is a no-op.
func (i *Instance) SetActivate(fn func()) {
	if i == nil {
		return
	}
	i.mu.Lock()
	i.activate = fn
	i.mu.Unlock()
}

// Close releases the lock (primary only). Safe on nil / non-primary handles.
func (i *Instance) Close() error {
	if !i.Primary() {
		return nil
	}
	if i.ln != nil {
		_ = i.ln.Close()
	}
	if i.lockPath != "" {
		_ = os.Remove(i.lockPath)
	}
	return nil
}

func (i *Instance) serve() {
	for {
		conn, err := i.ln.Accept()
		if err != nil {
			return // listener closed
		}
		go i.handle(conn)
	}
}

func (i *Instance) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return
	}
	got := strings.TrimSpace(line)
	if subtle.ConstantTimeCompare([]byte(got), []byte(i.token)) != 1 {
		return
	}
	fmt.Fprint(conn, "OK\n")

	i.mu.Lock()
	fn := i.activate
	i.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// signalExisting returns true only if a live primary completes the handshake.
func signalExisting(lockPath string) bool {
	port, token, err := readLock(lockPath)
	if err != nil {
		return false
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err != nil {
		return false // stale lock: nothing listening
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(1 * time.Second))

	fmt.Fprintf(conn, "%s\n", token)
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return false
	}
	return strings.TrimSpace(resp) == "OK"
}

func writeLock(path string, port int, token string) error {
	return os.WriteFile(path, fmt.Appendf(nil, "%d\n%s\n", port, token), 0o600)
}

func readLock(path string) (int, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("malformed lock file")
	}
	var port int
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &port); err != nil {
		return 0, "", err
	}
	return port, strings.TrimSpace(parts[1]), nil
}

func randToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
