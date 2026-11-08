package archive

import (
	"archive/zip"
	"github.com/PdB333/opencapsule/internal/capsule"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	root := t.TempDir()
	box := filepath.Join(root, "box")
	if err := os.MkdirAll(filepath.Join(box, "drive", "C"), 0700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(box, "drive", "C", "config.json"), []byte(`{"a":1}`), 0600)
	m := capsule.NewManifest("myapp")
	archive := filepath.Join(root, "myapp.ocap")
	if err := Pack(box, m, archive); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "out")
	got, err := Unpack(archive, out)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != m.ID {
		t.Fatal("ID mismatch")
	}
	data, err := os.ReadFile(filepath.Join(out, "drive", "C", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"a":1}` {
		t.Fatal(string(data))
	}
	if err := Pack(box, m, archive); err == nil {
		t.Fatal("overwrote archive")
	}
	if _, err := Unpack(archive, out); err == nil {
		t.Fatal("overwrote destination")
	}
}
func TestRejectTrickyNames(t *testing.T) {
	for _, name := range []string{"data/../outside", "/data/x", "data\\x", "data/a:b", "data//f", "data/./a", "data/a/../../x", ""} {
		if validName(name) {
			t.Fatalf("accepted %q", name)
		}
	}
}
func TestRejectZipSlip(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "evil.ocap")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, _ := z.Create("../escape")
	w.Write([]byte("bad"))
	z.Close()
	f.Close()
	if _, err := Unpack(out, filepath.Join(tmp, "unpacked")); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatal(err)
	}
}
func TestRejectSymlink(t *testing.T) {
	root := t.TempDir()
	box := filepath.Join(root, "box")
	os.Mkdir(box, 0700)
	if err := os.Symlink(filepath.Join(root, "secret"), filepath.Join(box, "link")); err != nil {
		t.Skip(err)
	}
	if err := Pack(box, capsule.NewManifest("test"), filepath.Join(root, "bad.ocap")); err == nil {
		t.Fatal("packed symlink")
	}
}
