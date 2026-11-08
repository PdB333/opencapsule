package capsule

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const SchemaVersion = 1

var validID = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,38}$`)

type Manifest struct {
	Schema  int       `json:"schema"`
	ID      string    `json:"id"`
	Box     string    `json:"box"`
	Entry   string    `json:"entry,omitempty"`
	Created time.Time `json:"created"`
	Backend string    `json:"backend"`
}

func ValidateID(id string) error {
	if !validID.MatchString(id) {
		return fmt.Errorf("invalid ID %q: use 2-39 lowercase characters, starting with a letter; a-z, 0-9, _ or -", id)
	}
	return nil
}

func BoxName(id string) string { return "OC_" + id }
func NewManifest(id string) Manifest {
	return Manifest{Schema: SchemaVersion, ID: id, Box: BoxName(id), Created: time.Now().UTC(), Backend: "sandboxie-plus"}
}
func (m Manifest) Validate() error {
	if m.Schema != SchemaVersion {
		return fmt.Errorf("unsupported manifest schema %d", m.Schema)
	}
	if err := ValidateID(m.ID); err != nil {
		return err
	}
	if m.Box != BoxName(m.ID) {
		return errors.New("box does not match ID")
	}
	if m.Backend != "sandboxie-plus" {
		return fmt.Errorf("unsupported backend %q", m.Backend)
	}
	if m.Created.IsZero() {
		return errors.New("missing created timestamp")
	}
	return nil
}

func DefaultRoot() (string, error) {
	if v := os.Getenv("OPENCAPSULE_HOME"); v != "" {
		return filepath.Abs(v)
	}
	if runtime.GOOS == "windows" {
		if base := os.Getenv("LOCALAPPDATA"); base != "" {
			return filepath.Join(base, "OpenCapsule"), nil
		}
		return "", errors.New("LOCALAPPDATA is undefined; set OPENCAPSULE_HOME")
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "OpenCapsule"), nil
}
func CapsuleDir(root, id string) string   { return filepath.Join(root, "capsules", id) }
func BoxDir(root, id string) string       { return filepath.Join(CapsuleDir(root, id), "box") }
func ManifestPath(root, id string) string { return filepath.Join(CapsuleDir(root, id), "capsule.json") }

func Load(root, id string) (Manifest, error) {
	var m Manifest
	if err := ValidateID(id); err != nil {
		return m, err
	}
	data, err := os.ReadFile(ManifestPath(root, id))
	if err != nil {
		return m, err
	}
	if err = json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	if err = m.Validate(); err != nil {
		return m, err
	}
	if m.ID != id {
		return m, errors.New("manifest ID differs from directory name")
	}
	return m, nil
}
func Write(root string, m Manifest) error {
	if err := m.Validate(); err != nil {
		return err
	}
	path := ManifestPath(root, m.ID)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return AtomicWrite(path, data, 0600)
}
func AtomicWrite(path string, data []byte, mode os.FileMode) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(parent, ".atomic-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// os.Rename uses a replace-existing move on Windows; never delete the old file first.
	return os.Rename(tmp, path)
}

// Owns reports whether child is strictly inside root, never root itself.
func Owns(root, child string) bool {
	rel, err := filepath.Rel(root, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Refuse to operate on capsule roots replaced with symlinks/junctions.
func VerifyRoot(root, id string) error {
	dir := CapsuleDir(root, id)
	if !Owns(filepath.Join(root, "capsules"), dir) {
		return errors.New("unsafe capsule path")
	}
	for _, p := range []string{filepath.Join(root, "capsules"), dir, BoxDir(root, id)} {
		info, err := os.Lstat(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("unsafe root component: %s", p)
		}
	}
	return nil
}
