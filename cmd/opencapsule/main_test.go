package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/PdB333/opencapsule/internal/archive"
	"github.com/PdB333/opencapsule/internal/capsule"
	"github.com/PdB333/opencapsule/internal/sandboxie"
)

func TestLifecycleWithFakeSandboxie(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake backend; real Windows integration covered in manual smoke tests")
	}
	root := t.TempDir()
	db := filepath.Join(root, "fake-db")
	if err := os.Mkdir(db, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_SBIE_DB", db)
	ini := filepath.Join(root, "ini.sh")
	start := filepath.Join(root, "start.sh")
	iniScript := `#!/bin/sh
set -eu
DB="$FAKE_SBIE_DB"
case "$1" in
query)
  if [ "$2" = "/boxes" ]; then
    for p in "$DB"/*_Enabled; do
      [ -f "$p" ] || continue
      [ "$(cat "$p")" = "y" ] || continue
      b="${p##*/}"
      b="${b%_Enabled}"
      echo "$b"
    done
    exit 0
  fi
  p="$DB/${2}_${3}"
  if [ -f "$p" ]; then cat "$p"; fi
  exit 0
  ;;
set)
  if [ "$3" = "*" ]; then
    rm -f "$DB/${2}_"*
    exit 0
  fi
  printf %s "$4" > "$DB/${2}_${3}"
  exit 0
  ;;
esac
exit 2
`
	startScript := `#!/bin/sh
case "${2:-}" in
/listpids) echo 0; exit 0 ;;
/terminate) exit 0 ;;
esac
exit 0
`
	if err := os.WriteFile(ini, []byte(iniScript), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(start, []byte(startScript), 0700); err != nil {
		t.Fatal(err)
	}
	e := sandboxie.Engine{Start: start, Ini: ini}
	m, err := create(root, e, "myapp", `C:\Program Files\Example\example.exe`)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Verify(m.Box, capsule.BoxDir(root, m.ID)); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(capsule.BoxDir(root, m.ID), "drive", "C", "settings.ini")
	if err := os.MkdirAll(filepath.Dir(data), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(data, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(root, "backup.ocap")
	if err := archive.Pack(capsule.BoxDir(root, m.ID), m, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(data, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.Idle(m.Box); err != nil {
		t.Fatal(err)
	}
	if err := restore(root, m, snapshot); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(data)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "original" {
		t.Fatalf("restore failed: %q", b)
	}
	cloned, err := importCapsule(root, e, "copy", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if cloned.ID != "copy" || cloned.Entry != m.Entry {
		t.Fatalf("bad imported manifest: %+v", cloned)
	}
	if err := e.Verify(cloned.Box, capsule.BoxDir(root, cloned.ID)); err != nil {
		t.Fatal(err)
	}
	if err := e.Remove(m.Box); err != nil {
		t.Fatal(err)
	}
	if err := e.Remove(cloned.Box); err != nil {
		t.Fatal(err)
	}
}
