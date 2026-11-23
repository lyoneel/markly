package markly

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// DefaultFilePerm is the mode Save and SaveAtomic give a file that does
// not exist yet, before the process umask applies. Existing files keep
// their own mode.
const DefaultFilePerm os.FileMode = 0o644

// maxSymlinkHops bounds symlink resolution in SaveAtomic, like the
// kernel's own loop limit.
const maxSymlinkHops = 40

// tempFileAttempts bounds the retries when a temp name already exists.
const tempFileAttempts = 10

// resolveWriteTarget follows path through any symlinks, including a
// dangling final link, and returns the regular file path to replace.
// Writing to the resolved path updates the link target and keeps the
// link, matching what Save (os.WriteFile) does.
func resolveWriteTarget(path string) (string, error) {
	target := path
	for range maxSymlinkHops {
		info, err := os.Lstat(target)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return target, nil
			}
			return "", err
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			return target, nil
		}
		link, err := os.Readlink(target)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(target), link)
		}
		target = link
	}
	return "", fmt.Errorf("resolve %s: too many levels of symbolic links", path)
}

// writeFileAtomic replaces path with data so that a crash leaves either
// the old or the new content:
//
//  1. a symlink at path is resolved and its target is replaced, so the
//     link stays a link;
//  2. data goes to a uniquely named temp file (".<name>.tmp-*") in the
//     target's directory, which receives the existing file's mode and,
//     on Unix, its owner and group where the process may set them, and
//     is fsynced;
//  3. the temp file is renamed over the target and the directory is
//     fsynced on POSIX systems (best effort).
//
// A new file gets newPerm, subject to umask. On any failure the temp
// file is removed and the target is not modified. Hard links to the old
// file keep the old content, as with any rename-based write.
func writeFileAtomic(path string, data []byte, newPerm os.FileMode) error {
	target, err := resolveWriteTarget(path)
	if err != nil {
		return err
	}
	dir, base := filepath.Dir(target), filepath.Base(target)

	existing, err := os.Stat(target)
	switch {
	case err == nil:
		if !existing.Mode().IsRegular() {
			return fmt.Errorf("cannot save: %s is not a regular file", target)
		}
	case errors.Is(err, fs.ErrNotExist):
		existing = nil
	default:
		return err
	}

	tmpPath, f, err := createSaveTemp(dir, base, newPerm)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = f.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := f.Write(data); err != nil {
		return err
	}
	if existing != nil {
		if err := f.Chmod(existing.Mode() & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)); err != nil {
			return err
		}
		keepOwner(f, existing)
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return err
	}
	committed = true
	syncDir(dir)
	return nil
}

// createSaveTemp exclusively creates a uniquely named temp file next to
// the target. The mode passes through umask like any new file.
func createSaveTemp(dir, base string, perm os.FileMode) (string, *os.File, error) {
	var lastErr error
	for range tempFileAttempts {
		name := filepath.Join(dir, "."+base+".tmp-"+rand.Text()[:12])
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm) // #nosec G302 G304 -- caller-chosen document path; DefaultFilePerm 0644 matches Save and umask applies
		if err == nil {
			return name, f, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", nil, err
		}
		lastErr = err
	}
	return "", nil, lastErr
}

// syncDir fsyncs a directory so a rename is durable. Best effort: some
// file systems and Windows do not support it, and the data itself is
// already synced.
func syncDir(dir string) {
	if runtime.GOOS == "windows" {
		return
	}
	df, err := os.Open(dir) // #nosec G304 -- directory of the document being saved
	if err != nil {
		return
	}
	_ = df.Sync()
	_ = df.Close()
}

// fileWriter returns a writer that saves relPath through dirly's atomic,
// policy-checked writer: a temp file plus rename inside the resolved
// folder root, keeping the file's mode and ownership.
func (mf *MDFolder) fileWriter(relPath string) func([]byte) error {
	return func(data []byte) error {
		return mf.dir.WriteFromBytes(relPath, data, DefaultFilePerm)
	}
}
