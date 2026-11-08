// Package archive implements a versioned, checksummed OpenCapsule ZIP format.
// It refuses path traversal, symbolic links and oversized/decompression-bomb inputs.
package archive

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/PdB333/opencapsule/internal/capsule"
)

const (
	maxEntries         = 200000
	maxBytes    uint64 = 8 << 30
	maxMetadata        = 32 << 20
)

type Checksums struct {
	Format int               `json:"format"`
	Files  map[string]string `json:"files"`
}

func validName(name string) bool {
	if name == "" || strings.ContainsAny(name, `\:`+"\x00") || strings.HasPrefix(name, "/") || path.Clean(name) != name {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
func archiveItems(zr *zip.ReadCloser) (map[string]*zip.File, error) {
	if len(zr.File) > maxEntries {
		return nil, errors.New("archive contains too many entries")
	}
	seen := make(map[string]*zip.File, len(zr.File))
	var total uint64
	for _, f := range zr.File {
		name := f.Name
		if strings.HasSuffix(name, "/") {
			continue
		} // directory headers are unnecessary
		if !validName(name) {
			return nil, fmt.Errorf("unsafe archive entry %q", name)
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate or case-colliding entry %q", name)
		}
		if !f.Mode().IsRegular() {
			return nil, fmt.Errorf("unsupported file type in %s", name)
		}
		if f.UncompressedSize64 > maxBytes-total {
			return nil, errors.New("archive exceeds size limit")
		}
		total += f.UncompressedSize64
		seen[key] = f
	}
	if seen["manifest.json"] == nil || seen["checksums.json"] == nil {
		return nil, errors.New("archive metadata missing")
	}
	return seen, nil
}
func readLimited(f *zip.File, max uint64) ([]byte, error) {
	if f.UncompressedSize64 > max {
		return nil, errors.New("archive metadata too large")
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, int64(max)+1))
}
func metadata(items map[string]*zip.File) (capsule.Manifest, Checksums, error) {
	var m capsule.Manifest
	var c Checksums
	b, err := readLimited(items["manifest.json"], maxMetadata)
	if err != nil {
		return m, c, err
	}
	if err = json.Unmarshal(b, &m); err != nil {
		return m, c, err
	}
	if err = m.Validate(); err != nil {
		return m, c, err
	}
	b, err = readLimited(items["checksums.json"], maxMetadata)
	if err != nil {
		return m, c, err
	}
	if err = json.Unmarshal(b, &c); err != nil {
		return m, c, err
	}
	if c.Format != 1 || c.Files == nil {
		return m, c, errors.New("unknown checksums format")
	}
	if len(c.Files) > maxEntries {
		return m, c, errors.New("too many checksums")
	}
	return m, c, nil
}
func ReadManifest(file string) (capsule.Manifest, error) {
	z, err := zip.OpenReader(file)
	if err != nil {
		return capsule.Manifest{}, err
	}
	defer z.Close()
	items, err := archiveItems(z)
	if err != nil {
		return capsule.Manifest{}, err
	}
	m, _, err := metadata(items)
	return m, err
}
func Pack(box string, m capsule.Manifest, target string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return errors.New("target exists; refusing to overwrite archive")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(target), ".ocap-partial-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	zw := zip.NewWriter(f)
	sums := Checksums{Format: 1, Files: map[string]string{}}
	writeMeta := func(name string, b []byte) error {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0600)
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err == nil {
		err = writeMeta("manifest.json", data)
	}
	if err != nil {
		zw.Close()
		f.Close()
		return err
	}
	count := 0
	var total uint64
	err = filepath.WalkDir(box, func(full string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if full == box {
			return nil
		}
		info, err := os.Lstat(full)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks/junctions are forbidden in capsule storage: %s", full)
		}
		if d.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("special file forbidden: %s", full)
		}
		count++
		if count > maxEntries {
			return errors.New("too many files")
		}
		if info.Size() < 0 || uint64(info.Size()) > maxBytes-total {
			return errors.New("capsule exceeds archive size limit")
		}
		total += uint64(info.Size())
		rel, err := filepath.Rel(box, full)
		if err != nil {
			return err
		}
		name := "data/" + filepath.ToSlash(rel)
		if !validName(name) {
			return fmt.Errorf("unsafe file path: %s", name)
		}
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0600)
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		input, err := os.Open(full)
		if err != nil {
			return err
		}
		defer input.Close()
		dig := sha256.New()
		n, err := io.Copy(io.MultiWriter(w, dig), input)
		if err != nil {
			return err
		}
		if n != info.Size() {
			return fmt.Errorf("file changed during export: %s", full)
		}
		sums.Files[name] = hex.EncodeToString(dig.Sum(nil))
		return nil
	})
	if err == nil {
		data, err = json.MarshalIndent(sums, "", "  ")
	}
	if err == nil {
		err = writeMeta("checksums.json", data)
	}
	cerr := zw.Close()
	if err == nil {
		err = cerr
	}
	cerr = f.Sync()
	if err == nil {
		err = cerr
	}
	cerr = f.Close()
	if err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	// Use no-clobber semantics where possible; a single controller lock also protects this target.
	if _, err := os.Stat(target); err == nil {
		return errors.New("archive target appeared during export")
	}
	return os.Rename(tmp, target)
}
func Unpack(file, destination string) (capsule.Manifest, error) {
	z, err := zip.OpenReader(file)
	if err != nil {
		return capsule.Manifest{}, err
	}
	defer z.Close()
	items, err := archiveItems(z)
	if err != nil {
		return capsule.Manifest{}, err
	}
	m, sums, err := metadata(items)
	if err != nil {
		return m, err
	}
	// Every file has one checksum, and every checksum has one data file.
	for _, f := range items {
		if f.Name == "manifest.json" || f.Name == "checksums.json" {
			continue
		}
		if !strings.HasPrefix(f.Name, "data/") {
			return m, fmt.Errorf("unexpected entry: %s", f.Name)
		}
		if _, ok := sums.Files[f.Name]; !ok {
			return m, fmt.Errorf("missing checksum: %s", f.Name)
		}
	}
	for name := range sums.Files {
		if items[strings.ToLower(name)] == nil {
			return m, fmt.Errorf("missing data: %s", name)
		}
	}
	if len(sums.Files) != len(items)-2 {
		return m, errors.New("checksum count mismatch")
	}
	if err := os.Mkdir(destination, 0700); err != nil {
		return m, fmt.Errorf("destination must not exist: %w", err)
	}
	for _, f := range z.File {
		if f.Name == "manifest.json" || f.Name == "checksums.json" || strings.HasSuffix(f.Name, "/") {
			continue
		}
		rel := strings.TrimPrefix(f.Name, "data/")
		out := filepath.Join(destination, filepath.FromSlash(rel))
		if !capsule.Owns(destination, out) {
			return m, fmt.Errorf("outside destination: %s", f.Name)
		}
		if err := os.MkdirAll(filepath.Dir(out), 0700); err != nil {
			return m, err
		}
		r, err := f.Open()
		if err != nil {
			return m, err
		}
		to, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			r.Close()
			return m, err
		}
		dig := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(to, dig), io.LimitReader(r, int64(f.UncompressedSize64)+1))
		closeErr := to.Close()
		r.Close()
		if copyErr != nil {
			return m, copyErr
		}
		if closeErr != nil {
			return m, closeErr
		}
		if n != int64(f.UncompressedSize64) {
			return m, fmt.Errorf("bad size: %s", f.Name)
		}
		if !strings.EqualFold(hex.EncodeToString(dig.Sum(nil)), sums.Files[f.Name]) {
			return m, fmt.Errorf("checksum mismatch: %s", f.Name)
		}
	}
	return m, nil
}
