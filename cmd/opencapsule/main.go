package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PdB333/opencapsule/internal/archive"
	"github.com/PdB333/opencapsule/internal/capsule"
	"github.com/PdB333/opencapsule/internal/sandboxie"
)

const version = "1.0.0-preview.1"

const usage = `OpenCapsule — conteneurs Win32 persistants et legers (backend Sandboxie Plus)
Usage:
  opencapsule doctor
  opencapsule create [--entry "C:\\path\\app.exe"] NAME
  opencapsule list
  opencapsule inspect NAME
  opencapsule set-entry NAME "C:\\path\\app.exe"
  opencapsule install [--fake-admin] NAME "C:\\path\\setup.exe" [installer args...]
  opencapsule run [--wait] NAME [--] [exe [args...]]
  opencapsule shell NAME
  opencapsule stop NAME
  opencapsule snapshot NAME
  opencapsule export NAME FILE.ocap
  opencapsule restore NAME FILE.ocap
  opencapsule import NEW_NAME FILE.ocap
  opencapsule verify FILE.ocap
  opencapsule delete --yes NAME
  opencapsule version

Environnement: OPENCAPSULE_HOME (stockage), OPENCAPSULE_SBIE_DIR (Start.exe et SbieIni.exe).
IMPORTANT: ce n'est PAS une frontiere de securite contre un pilote kernel malveillant.
`

func fatal(err error) { fmt.Fprintln(os.Stderr, "error:", err); os.Exit(1) }
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fatal(err)
	}
}
func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	command := args[0]
	a := args[1:]
	if command == "help" || command == "-h" || command == "--help" {
		fmt.Fprint(out, usage)
		return nil
	}
	if command == "version" {
		fmt.Fprintln(out, version)
		return nil
	}
	root, err := capsule.DefaultRoot()
	if err != nil {
		return err
	}
	if command == "list" {
		if len(a) != 0 {
			return errors.New("usage: list")
		}
		dir := filepath.Join(root, "capsules")
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		var ids []string
		for _, item := range entries {
			if item.IsDir() {
				ids = append(ids, item.Name())
			}
		}
		sort.Strings(ids)
		for _, id := range ids {
			m, err := capsule.Load(root, id)
			if err != nil {
				fmt.Fprintf(out, "%-24s (invalid: %v)\n", id, err)
				continue
			}
			entry := m.Entry
			if entry == "" {
				entry = "(not configured)"
			}
			fmt.Fprintf(out, "%-24s %-15s %s\n", m.ID, m.Backend, entry)
		}
		return nil
	}
	if command == "inspect" {
		if len(a) != 1 {
			return errors.New("usage: inspect NAME")
		}
		m, err := capsule.Load(root, a[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "ID: %s\nBox: %s\nEntry: %s\nCreated: %s\nStorage: %s\nBackend: %s\n", m.ID, m.Box, m.Entry, m.Created.Format(time.RFC3339), capsule.BoxDir(root, m.ID), m.Backend)
		return nil
	}
	if command == "set-entry" {
		if len(a) != 2 || strings.TrimSpace(a[1]) == "" {
			return errors.New("usage: set-entry NAME EXE")
		}
		unlock, err := acquire(root)
		if err != nil {
			return err
		}
		defer unlock()
		m, err := capsule.Load(root, a[0])
		if err != nil {
			return err
		}
		m.Entry = a[1]
		if err := capsule.Write(root, m); err != nil {
			return err
		}
		fmt.Fprintln(out, "Saved entry for", m.ID)
		return nil
	}
	if command == "verify" {
		if len(a) != 1 {
			return errors.New("usage: verify FILE.ocap")
		}
		tmp, err := os.MkdirTemp("", "ocap-verify-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		m, err := archive.Unpack(a[0], filepath.Join(tmp, "extract"))
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "OK: %s (schema %d, checksums valid)\n", m.ID, m.Schema)
		return nil
	}
	// Everything below requires the official Sandboxie backend on Windows.
	if err := sandboxie.Supported(); err != nil {
		return err
	}
	eng, err := sandboxie.Locate()
	if err != nil {
		return err
	}
	if command == "doctor" {
		fmt.Fprintf(out, "OpenCapsule: %s\nStart.exe: %s\nSbieIni.exe: %s\nHome: %s\n", version, eng.Start, eng.Ini, root)
		return nil
	}
	switch command {
	case "create":
		f := flag.NewFlagSet("create", flag.ContinueOnError)
		f.SetOutput(io.Discard)
		entry := f.String("entry", "", "executable to run later")
		if err := f.Parse(a); err != nil {
			return err
		}
		if f.NArg() != 1 {
			return errors.New("usage: create [--entry EXE] NAME")
		}
		unlock, err := acquire(root)
		if err != nil {
			return err
		}
		defer unlock()
		m, err := create(root, eng, f.Arg(0), *entry)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Created %s (Sandboxie box %s)\n", m.ID, m.Box)
	case "install":
		f := flag.NewFlagSet("install", flag.ContinueOnError)
		f.SetOutput(io.Discard)
		fake := f.Bool("fake-admin", false, "simulate admin token without host elevation")
		if err := f.Parse(a); err != nil {
			return err
		}
		args := f.Args()
		if len(args) < 2 {
			return errors.New("usage: install [--fake-admin] NAME INSTALLER [args...]")
		}
		m, err := managed(root, eng, args[0])
		if err != nil {
			return err
		}
		executable, installArgs := args[1], args[2:]
		if strings.EqualFold(filepath.Ext(executable), ".msi") {
			installArgs = append([]string{"/i", executable}, installArgs...)
			executable = "msiexec.exe"
		}
		if err := eng.RunOptions(m.Box, executable, installArgs, true, *fake); err != nil {
			return err
		}
		fmt.Fprintln(out, "Installer exited; child processes or MSI services may still be active. Check with Sandboxie Plus, then set-entry.")
	case "run":
		f := flag.NewFlagSet("run", flag.ContinueOnError)
		f.SetOutput(io.Discard)
		wait := f.Bool("wait", false, "wait for process")
		if err := f.Parse(a); err != nil {
			return err
		}
		rest := f.Args()
		if len(rest) >= 2 && rest[1] == "--" {
			rest = append(rest[:1:1], rest[2:]...)
		}
		if len(rest) < 1 {
			return errors.New("usage: run [--wait] NAME [--] [exe [args...]]")
		}
		m, err := managed(root, eng, rest[0])
		if err != nil {
			return err
		}
		exe := m.Entry
		var exeArgs []string
		if len(rest) >= 2 {
			exe = rest[1]
			exeArgs = rest[2:]
		}
		if exe == "" {
			return errors.New("no entry configured; run NAME -- EXE or set-entry NAME EXE")
		}
		if err := eng.Run(m.Box, exe, exeArgs, *wait); err != nil {
			return err
		}
		fmt.Fprintln(out, "Launch requested in", m.Box)
	case "shell":
		if len(a) != 1 {
			return errors.New("usage: shell NAME")
		}
		m, err := managed(root, eng, a[0])
		if err != nil {
			return err
		}
		return eng.Run(m.Box, "run_dialog", nil, false)
	case "stop":
		if len(a) != 1 {
			return errors.New("usage: stop NAME")
		}
		m, err := managed(root, eng, a[0])
		if err != nil {
			return err
		}
		if err := eng.Stop(m.Box); err != nil {
			return err
		}
		fmt.Fprintln(out, "Termination requested; check remaining processes before snapshots.")
	case "snapshot", "export":
		if (command == "snapshot" && len(a) != 1) || (command == "export" && len(a) != 2) {
			return fmt.Errorf("usage: %s NAME %s", command, map[bool]string{true: "", false: "FILE.ocap"}[command == "snapshot"])
		}
		unlock, err := acquire(root)
		if err != nil {
			return err
		}
		defer unlock()
		m, err := managed(root, eng, a[0])
		if err != nil {
			return err
		}
		if err := eng.Idle(m.Box); err != nil {
			return err
		}
		target := ""
		if command == "export" {
			target, err = filepath.Abs(a[1])
			if err != nil {
				return err
			}
		} else {
			target = filepath.Join(capsule.CapsuleDir(root, m.ID), "snapshots", time.Now().UTC().Format("20060102T150405.000000000")+".ocap")
		}
		if capsule.Owns(capsule.BoxDir(root, m.ID), target) {
			return errors.New("cannot export into sandbox data directory")
		}
		if err := archive.Pack(capsule.BoxDir(root, m.ID), m, target); err != nil {
			return err
		}
		fmt.Fprintln(out, target)
	case "restore":
		if len(a) != 2 {
			return errors.New("usage: restore NAME FILE.ocap")
		}
		unlock, err := acquire(root)
		if err != nil {
			return err
		}
		defer unlock()
		m, err := managed(root, eng, a[0])
		if err != nil {
			return err
		}
		if err := eng.Idle(m.Box); err != nil {
			return err
		}
		src, err := archive.ReadManifest(a[1])
		if err != nil {
			return err
		}
		if src.ID != m.ID {
			return fmt.Errorf("snapshot belongs to %s, not %s", src.ID, m.ID)
		}
		if err := restore(root, m, a[1]); err != nil {
			return err
		}
		fmt.Fprintln(out, "Restored", m.ID)
	case "import":
		if len(a) != 2 {
			return errors.New("usage: import NEW_NAME FILE.ocap")
		}
		unlock, err := acquire(root)
		if err != nil {
			return err
		}
		defer unlock()
		m, err := importCapsule(root, eng, a[0], a[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Imported %s (from archive)\n", m.ID)
	case "delete":
		if len(a) != 2 || a[0] != "--yes" {
			return errors.New("usage: delete --yes NAME (permanently destroys managed capsule)")
		}
		unlock, err := acquire(root)
		if err != nil {
			return err
		}
		defer unlock()
		m, err := managed(root, eng, a[1])
		if err != nil {
			return err
		}
		if err := eng.Idle(m.Box); err != nil {
			return err
		}
		if err := capsule.VerifyRoot(root, m.ID); err != nil {
			return err
		}
		if err := eng.Remove(m.Box); err != nil {
			return err
		}
		if err := os.RemoveAll(capsule.CapsuleDir(root, m.ID)); err != nil {
			return err
		}
		fmt.Fprintln(out, "Deleted", m.ID)
	default:
		return fmt.Errorf("unknown command %q\n%s", command, usage)
	}
	return nil
}

func managed(root string, e sandboxie.Engine, id string) (capsule.Manifest, error) {
	m, err := capsule.Load(root, id)
	if err != nil {
		return m, err
	}
	if err := capsule.VerifyRoot(root, id); err != nil {
		return m, err
	}
	if err := e.Verify(m.Box, capsule.BoxDir(root, id)); err != nil {
		return m, err
	}
	return m, nil
}
func create(root string, e sandboxie.Engine, id, entry string) (capsule.Manifest, error) {
	m := capsule.NewManifest(id)
	if err := m.Validate(); err != nil {
		return m, err
	}
	if err := capsule.VerifyRoot(root, id); err != nil {
		return m, err
	}
	dir := capsule.CapsuleDir(root, id)
	if err := os.MkdirAll(filepath.Dir(dir), 0700); err != nil {
		return m, err
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return m, fmt.Errorf("capsule already exists or cannot be created: %w", err)
	}
	if err := os.Mkdir(capsule.BoxDir(root, id), 0700); err != nil {
		os.RemoveAll(dir)
		return m, err
	}
	m.Entry = entry
	if err := e.Configure(m.Box, capsule.BoxDir(root, id)); err != nil {
		return m, fmt.Errorf("Sandboxie setup incomplete, retained %s for inspection: %w", dir, err)
	}
	if err := capsule.Write(root, m); err != nil {
		return m, fmt.Errorf("manifest write failed; box was created, recovery required: %w", err)
	}
	return m, nil
}
func restore(root string, m capsule.Manifest, file string) error {
	dir := capsule.CapsuleDir(root, m.ID)
	temp := filepath.Join(dir, "box-restore-pending")
	previous := filepath.Join(dir, "box-previous")
	if _, err := os.Lstat(temp); err == nil {
		return errors.New("stale restore staging directory exists")
	}
	if _, err := os.Lstat(previous); err == nil {
		return errors.New("stale previous directory exists")
	}
	defer os.RemoveAll(temp)
	if _, err := archive.Unpack(file, temp); err != nil {
		return err
	}
	box := capsule.BoxDir(root, m.ID)
	if err := os.Rename(box, previous); err != nil {
		return err
	}
	if err := os.Rename(temp, box); err != nil {
		rollbackErr := os.Rename(previous, box)
		if rollbackErr != nil {
			return fmt.Errorf("restore failed: %v; rollback also failed: %v", err, rollbackErr)
		}
		return err
	}
	if err := os.RemoveAll(previous); err != nil {
		return fmt.Errorf("restored but previous data remains at %s: %w", previous, err)
	}
	return nil
}
func importCapsule(root string, e sandboxie.Engine, id, file string) (capsule.Manifest, error) {
	m := capsule.NewManifest(id)
	if err := m.Validate(); err != nil {
		return m, err
	}
	if err := capsule.VerifyRoot(root, id); err != nil {
		return m, err
	}
	original, err := archive.ReadManifest(file)
	if err != nil {
		return m, err
	}
	dir := capsule.CapsuleDir(root, id)
	if err := os.MkdirAll(filepath.Dir(dir), 0700); err != nil {
		return m, err
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return m, err
	}
	fromZip, err := archive.Unpack(file, capsule.BoxDir(root, id))
	if err != nil {
		os.RemoveAll(dir)
		return m, err
	}
	if fromZip.ID != original.ID {
		os.RemoveAll(dir)
		return m, errors.New("archive manifest changed")
	}
	m.Entry = original.Entry
	if err := e.Configure(m.Box, capsule.BoxDir(root, id)); err != nil {
		return m, fmt.Errorf("Sandboxie setup incomplete, retained %s for inspection: %w", dir, err)
	}
	if err := capsule.Write(root, m); err != nil {
		return m, err
	}
	return m, nil
}
func acquire(root string) (func(), error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	p := filepath.Join(root, ".lock")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("OpenCapsule busy or stale lock at %s: %w", p, err)
	}
	fmt.Fprintf(f, "pid=%d\ntime=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	f.Close()
	return func() { os.Remove(p) }, nil
}
