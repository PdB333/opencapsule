// Package sandboxie controls an existing Sandboxie Plus installation.
// It never replaces or patches Sandboxie binaries or its global configuration file.
package sandboxie

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf16"
)

type Engine struct {
	Start string
	Ini   string
}

func Locate() (Engine, error) {
	var dirs []string
	if d := os.Getenv("OPENCAPSULE_SBIE_DIR"); d != "" {
		dirs = append(dirs, d)
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
		if base := os.Getenv(env); base != "" {
			dirs = append(dirs, filepath.Join(base, "Sandboxie-Plus"), filepath.Join(base, "Sandboxie"))
		}
	}
	for _, d := range dirs {
		st, ini := filepath.Join(d, "Start.exe"), filepath.Join(d, "SbieIni.exe")
		if isFile(st) && isFile(ini) {
			return Engine{Start: st, Ini: ini}, nil
		}
	}
	return Engine{}, errors.New("Sandboxie Plus non trouve : installe Sandboxie Plus ou definis OPENCAPSULE_SBIE_DIR")
}
func isFile(p string) bool { info, err := os.Stat(p); return err == nil && !info.IsDir() }

func textOutput(b []byte) string {
	if len(b) >= 2 && ((b[0] == 0xff && b[1] == 0xfe) || (b[0] == 0xfe && b[1] == 0xff)) {
		little := b[0] == 0xff
		a := make([]uint16, 0, (len(b)-2)/2)
		for i := 2; i+1 < len(b); i += 2 {
			if little {
				a = append(a, uint16(b[i])|uint16(b[i+1])<<8)
			} else {
				a = append(a, uint16(b[i+1])|uint16(b[i])<<8)
			}
		}
		return string(utf16.Decode(a))
	}
	// Some Windows tools emit UTF-16 LE without a BOM when output is piped.
	if len(b) >= 4 && b[1] == 0 && b[3] == 0 {
		a := make([]uint16, 0, len(b)/2)
		for i := 0; i+1 < len(b); i += 2 {
			a = append(a, uint16(b[i])|uint16(b[i+1])<<8)
		}
		return string(utf16.Decode(a))
	}
	return string(b)
}
func call(path string, args ...string) (string, error) {
	cmd := exec.Command(path, args...)
	out, err := cmd.CombinedOutput()
	result := strings.TrimSpace(textOutput(out))
	if err != nil {
		return result, fmt.Errorf("%s %v: %w; output: %s", filepath.Base(path), args, err, result)
	}
	return result, nil
}
func (e Engine) value(box, key string) (string, error) {
	s, err := call(e.Ini, "query", box, key)
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	// Some editions print Key=Value; the official CLI prints only Value.
	if strings.HasPrefix(strings.ToLower(s), strings.ToLower(key)+"=") {
		s = s[len(key)+1:]
	}
	return strings.TrimSpace(s), nil
}
func (e Engine) set(box, key, value string) error {
	_, err := call(e.Ini, "set", box, key, value)
	if err != nil {
		return err
	}
	actual, err := e.value(box, key)
	if err != nil {
		return err
	}
	if key == "FileRootPath" {
		if !strings.EqualFold(filepath.Clean(actual), filepath.Clean(value)) {
			return fmt.Errorf("Sandboxie did not retain %s: got %q expected %q", key, actual, value)
		}
	} else if !strings.EqualFold(actual, value) {
		return fmt.Errorf("Sandboxie did not retain %s: got %q expected %q", key, actual, value)
	}
	return nil
}
func (e Engine) existingBoxes() (map[string]bool, error) {
	out, err := call(e.Ini, "query", "/boxes", "*")
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, s := range strings.FieldsFunc(out, func(r rune) bool { return r == '\n' || r == '\r' }) {
		s = strings.TrimSpace(s)
		if s != "" {
			names[strings.ToLower(s)] = true
		}
	}
	return names, nil
}
func (e Engine) Configure(box, storage string) error {
	if !strings.HasPrefix(box, "OC_") {
		return errors.New("refusing non-OpenCapsule box")
	}
	names, err := e.existingBoxes()
	if err != nil {
		return err
	}
	if names[strings.ToLower(box)] {
		return fmt.Errorf("Sandboxie box %s already exists", box)
	}
	// Keep Enabled last. A partially configured box must not be runnable.
	for _, kv := range [][2]string{{"FileRootPath", storage}, {"AutoDelete", "n"}, {"AutoRemove", "n"}, {"DropAdminRights", "y"}, {"FakeAdminRights", "n"}, {"Enabled", "y"}} {
		if err := e.set(box, kv[0], kv[1]); err != nil {
			return fmt.Errorf("config %s: %w", kv[0], err)
		}
	}
	return nil
}
func (e Engine) Verify(box, storage string) error {
	for _, kv := range [][2]string{{"FileRootPath", storage}, {"AutoDelete", "n"}, {"Enabled", "y"}, {"DropAdminRights", "y"}} {
		got, err := e.value(box, kv[0])
		if err != nil {
			return err
		}
		if !strings.EqualFold(filepath.Clean(got), filepath.Clean(kv[1])) {
			return fmt.Errorf("Sandboxie setting %s mismatch: got %q want %q", kv[0], got, kv[1])
		}
	}
	return nil
}
func (e Engine) Remove(box string) error {
	if !strings.HasPrefix(box, "OC_") {
		return errors.New("refusing non-OpenCapsule box")
	}
	// SbieIni "set BoxName *" removes the *configuration section*, not the data.
	if _, err := call(e.Ini, "set", box, "*"); err != nil {
		return err
	}
	names, err := e.existingBoxes()
	if err != nil {
		return err
	}
	if names[strings.ToLower(box)] {
		return errors.New("sandbox definition still present after removal")
	}
	return nil
}
func (e Engine) Run(box, program string, args []string, wait bool) error {
	return e.RunOptions(box, program, args, wait, false)
}
func (e Engine) RunOptions(box, program string, args []string, wait, fakeAdmin bool) error {
	if program == "" {
		return errors.New("missing program path")
	}
	argv := []string{"/box:" + box}
	if wait {
		argv = append(argv, "/wait")
	}
	if fakeAdmin {
		argv = append(argv, "/fake_admin")
	}
	argv = append(argv, program)
	argv = append(argv, args...)
	cmd := exec.Command(e.Start, argv...)
	if wait {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("sandboxed process failed: %w", err)
		}
		return nil
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
func (e Engine) Stop(box string) error {
	_, err := call(e.Start, "/box:"+box, "/terminate")
	return err
}
func (e Engine) PIDs(box string) ([]int, error) {
	output, err := call(e.Start, "/box:"+box, "/listpids")
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return nil, errors.New("listpids returned empty output; refusing unsafe operation")
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil || n < 0 || n > 100000 {
		return nil, fmt.Errorf("unexpected listpids output: %q", output)
	}
	if len(fields) < n+1 {
		return nil, fmt.Errorf("incomplete listpids output: %q", output)
	}
	pids := make([]int, 0, n)
	for _, v := range fields[1 : n+1] {
		pid, err := strconv.Atoi(v)
		if err != nil || pid <= 0 {
			return nil, fmt.Errorf("bad PID: %q", v)
		}
		pids = append(pids, pid)
	}
	return pids, nil
}
func (e Engine) Idle(box string) error {
	pids, err := e.PIDs(box)
	if err != nil {
		return err
	}
	if len(pids) > 0 {
		return fmt.Errorf("%d process(es) still running in %s: %v", len(pids), box, pids)
	}
	return nil
}
func Supported() error {
	if runtime.GOOS != "windows" {
		return errors.New("execution requires Windows 10/11 x64; file/manifest tests are cross-platform")
	}
	return nil
}
