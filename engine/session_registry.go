package engine

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SessionEntry is one named, auto-managed Chrome in the session registry.
// A session is a long-lived `serve` process bound to a disk profile of the
// same name (so cookies persist), reused across CLI invocations.
type SessionEntry struct {
	Name       string `json:"name"`
	Port       int    `json:"port"`
	PID        int    `json:"pid"`
	WSURL      string `json:"ws_url"`
	Profile    string `json:"profile"`
	LaunchedAt string `json:"launched_at"`
}

// sessionRegistry is the on-disk map of name → SessionEntry.
type sessionRegistry struct {
	Sessions map[string]SessionEntry `json:"sessions"`
}

// SessionSpawnOpts carries the global flags propagated to a spawned serve.
type SessionSpawnOpts struct {
	Headless bool
	Stealth  bool
	Proxy    string
}

var sessionNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func validateSessionName(name string) error {
	if name == "" {
		return fmt.Errorf("session name is empty")
	}
	if !sessionNameRe.MatchString(name) {
		return fmt.Errorf("invalid session name %q (allowed: letters, digits, '-', '_')", name)
	}
	return nil
}

func sessionRegistryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	dir := filepath.Join(home, ".ghostchrome")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create .ghostchrome dir: %w", err)
	}
	return filepath.Join(dir, "sessions.json"), nil
}

func sessionLogPath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".ghostchrome", "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".log"), nil
}

func loadSessionRegistry() (*sessionRegistry, string, error) {
	path, err := sessionRegistryPath()
	if err != nil {
		return nil, "", err
	}
	reg := &sessionRegistry{Sessions: map[string]SessionEntry{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, path, nil
		}
		return nil, path, fmt.Errorf("read sessions registry: %w", err)
	}
	if err := json.Unmarshal(data, reg); err != nil {
		return nil, path, fmt.Errorf("parse sessions registry: %w", err)
	}
	if reg.Sessions == nil {
		reg.Sessions = map[string]SessionEntry{}
	}
	return reg, path, nil
}

func saveSessionRegistry(path string, reg *sessionRegistry) error {
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// sessionAlive probes the session's CDP port. It returns the (fresh) browser
// WebSocket URL and true when Chrome answers, false otherwise. Liveness is
// authoritative via CDP rather than the stored PID: if our serve is gone the
// port stops answering.
func sessionAlive(e SessionEntry) (string, bool) {
	ws, err := DiscoverCDP([]int{e.Port}, 600*time.Millisecond)
	if err != nil || ws == "" {
		return "", false
	}
	return ws, true
}

// cmdlineIsSessionServe reports whether a process command line is the serve
// that backs this session. Identity is the spawn fingerprint matched as EXACT
// tokens: the `serve` subcommand AND `--port <port>` AND `--user-profile
// <name>`. The exact (random) port + the session name make this unique to our
// process without depending on the binary's name (it may be renamed/symlinked).
func cmdlineIsSessionServe(cmd string, e SessionEntry) bool {
	fields := strings.Fields(cmd)
	serve := false
	port := strconv.Itoa(e.Port)
	portOK := false
	nameOK := false
	for i, f := range fields {
		if f == "serve" {
			serve = true
		}
		switch {
		case f == "--port" && i+1 < len(fields) && fields[i+1] == port:
			portOK = true
		case f == "--port="+port:
			portOK = true
		case f == "--user-profile" && i+1 < len(fields) && fields[i+1] == e.Name:
			nameOK = true
		case f == "--user-profile="+e.Name:
			nameOK = true
		}
	}
	return serve && portOK && nameOK
}

// killSessionProcess signals the session's serve process ONLY if the PID still
// belongs to THIS session's ghostchrome serve. Guards against the OS having
// recycled a dead serve's PID for an unrelated process — we must never signal
// an innocent process during respawn/prune/stop. processCmdline is provided
// per-platform (session_spawn_{unix,windows}.go).
func killSessionProcess(e SessionEntry) {
	cmd, ok := processCmdline(e.PID)
	if !ok {
		return // gone or unverifiable — do not signal anything
	}
	if !cmdlineIsSessionServe(cmd, e) {
		return // PID reused by an unrelated process — leave it alone
	}
	_ = killPID(e.PID)
}

// freePort asks the OS for an unused localhost TCP port.
func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// AcquireSession resolves a named session to a browser WebSocket URL. If the
// session is already running it is reused; otherwise a detached `serve`
// process is spawned, bound to the disk profile of the same name, and probed
// until its CDP endpoint is ready.
func AcquireSession(name string, opts SessionSpawnOpts) (string, error) {
	if err := validateSessionName(name); err != nil {
		return "", err
	}
	reg, path, err := loadSessionRegistry()
	if err != nil {
		return "", err
	}

	if entry, ok := reg.Sessions[name]; ok {
		if ws, alive := sessionAlive(entry); alive {
			return ws, nil
		}
		// CDP-dead, but the serve process may still linger (e.g. Chrome
		// crashed and serve hasn't polled yet). Kill it so we never orphan
		// the old serve when spawning its replacement.
		killSessionProcess(entry)
		delete(reg.Sessions, name)
	}

	port, err := freePort()
	if err != nil {
		return "", fmt.Errorf("allocate port: %w", err)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate ghostchrome binary: %w", err)
	}

	args := []string{
		"serve",
		"--port", strconv.Itoa(port),
		"--user-profile", name,
		fmt.Sprintf("--headless=%t", opts.Headless),
	}
	if opts.Stealth {
		args = append(args, "--stealth")
	}
	if opts.Proxy != "" {
		args = append(args, "--proxy", opts.Proxy)
	}

	logPath, err := sessionLogPath(name)
	if err != nil {
		return "", err
	}
	logFile, err := os.Create(logPath)
	if err != nil {
		return "", fmt.Errorf("open session log: %w", err)
	}

	cmd := exec.Command(exe, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = detachSysProcAttr()
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return "", fmt.Errorf("spawn session serve: %w", err)
	}
	pid := cmd.Process.Pid
	// Detach: let the serve outlive this CLI process. The child keeps its own
	// copy of the log fd, so the parent can close its handle.
	_ = cmd.Process.Release()
	logFile.Close()

	ws, err := waitForCDP(port, 15*time.Second)
	if err != nil {
		_ = killPID(pid)
		return "", fmt.Errorf("session %q: chrome did not come up (see %s): %w", name, logPath, err)
	}

	reg.Sessions[name] = SessionEntry{
		Name:       name,
		Port:       port,
		PID:        pid,
		WSURL:      ws,
		Profile:    name,
		LaunchedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := saveSessionRegistry(path, reg); err != nil {
		return "", fmt.Errorf("save sessions registry: %w", err)
	}
	return ws, nil
}

// waitForCDP polls the port until Chrome's /json/version answers or timeout.
func waitForCDP(port int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ws, err := DiscoverCDP([]int{port}, 500*time.Millisecond); err == nil && ws != "" {
			return ws, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", fmt.Errorf("timeout after %s", timeout)
}

// SessionRegistryEntry is the public-facing representation used by the CLI.
type SessionRegistryEntry struct {
	Name       string `json:"name"`
	Port       int    `json:"port"`
	PID        int    `json:"pid"`
	WSURL      string `json:"ws_url"`
	Profile    string `json:"profile"`
	LaunchedAt string `json:"launched_at,omitempty"`
	Alive      bool   `json:"alive"`
}

// AliveStr returns "yes" or "no".
func (e SessionRegistryEntry) AliveStr() string {
	if e.Alive {
		return "yes"
	}
	return "no"
}

// ListSessions returns every registry entry annotated with current liveness.
func ListSessions() ([]SessionRegistryEntry, error) {
	reg, _, err := loadSessionRegistry()
	if err != nil {
		return nil, err
	}
	out := make([]SessionRegistryEntry, 0, len(reg.Sessions))
	for name, e := range reg.Sessions {
		_, alive := sessionAlive(e)
		out = append(out, SessionRegistryEntry{
			Name:       name,
			Port:       e.Port,
			PID:        e.PID,
			WSURL:      e.WSURL,
			Profile:    e.Profile,
			LaunchedAt: e.LaunchedAt,
			Alive:      alive,
		})
	}
	return out, nil
}

// StopSession terminates the named session's process and removes it from the
// registry (best-effort: the process may already be gone).
func StopSession(name string) error {
	reg, path, err := loadSessionRegistry()
	if err != nil {
		return err
	}
	entry, ok := reg.Sessions[name]
	if !ok {
		return fmt.Errorf("session %q not found in registry", name)
	}
	killSessionProcess(entry)
	delete(reg.Sessions, name)
	if lp, err := sessionLogPath(name); err == nil {
		_ = os.Remove(lp)
	}
	return saveSessionRegistry(path, reg)
}

// PruneSessions removes registry entries whose Chrome is no longer reachable,
// killing any lingering serve process and its log. Returns the count pruned.
func PruneSessions() (int, error) {
	reg, path, err := loadSessionRegistry()
	if err != nil {
		return 0, err
	}
	n := 0
	for name, entry := range reg.Sessions {
		if _, alive := sessionAlive(entry); alive {
			continue
		}
		killSessionProcess(entry)
		if lp, lerr := sessionLogPath(name); lerr == nil {
			_ = os.Remove(lp)
		}
		delete(reg.Sessions, name)
		n++
	}
	if n > 0 {
		if err := saveSessionRegistry(path, reg); err != nil {
			return n, err
		}
	}
	return n, nil
}

// KillAllSessions stops every registered session.
func KillAllSessions() (int, error) {
	reg, path, err := loadSessionRegistry()
	if err != nil {
		return 0, err
	}
	n := 0
	for name, entry := range reg.Sessions {
		killSessionProcess(entry)
		if lp, lerr := sessionLogPath(name); lerr == nil {
			_ = os.Remove(lp)
		}
		n++
	}
	reg.Sessions = map[string]SessionEntry{}
	if err := saveSessionRegistry(path, reg); err != nil {
		return n, err
	}
	return n, nil
}
