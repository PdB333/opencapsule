package capsule

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIDs(t *testing.T) {
	for _, id := range []string{"myapp", "a1", "a_test", "a-b"} {
		if err := ValidateID(id); err != nil {
			t.Errorf("%s: %v", id, err)
		}
	}
	for _, id := range []string{"A", "a", "../esc", "a/b", "a\\b", "1bad", "hello world", "CON:"} {
		if ValidateID(id) == nil {
			t.Errorf("accepted %q", id)
		}
	}
}
func TestManifest(t *testing.T) {
	root := t.TempDir()
	m := NewManifest("myapp")
	if err := Write(root, m); err != nil {
		t.Fatal(err)
	}
	got, err := Load(root, "myapp")
	if err != nil {
		t.Fatal(err)
	}
	if got.Box != "OC_myapp" {
		t.Fatal(got)
	}
	if !Owns(root, filepath.Join(root, "x")) || Owns(root, root) || Owns(root, filepath.Join(root, "..", "escape")) {
		t.Fatal("Owns boundary")
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "capsules", "myapp", "box")); err == nil {
		if VerifyRoot(root, "myapp") == nil {
			t.Fatal("followed symlink")
		}
	}
}
