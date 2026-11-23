package markly

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
)

// DefaultMaxFileSize is the largest document, in bytes, that MDFile
// and MDFolder read from disk unless WithMaxFileSize or
// WithFolderMaxFileSize sets another limit. It is four times
// MaxLineLength, so a document with one maximal line still loads.
const DefaultMaxFileSize int64 = 64 << 20

// ErrFileTooLarge is wrapped by read errors for a document larger than
// its size limit (DefaultMaxFileSize unless configured).
var ErrFileTooLarge = errors.New("file too large")

// WithMaxFileSize sets the largest document, in bytes, the MDFile
// reads from disk. A file whose size exceeds the limit, or that grows
// past it while being read, fails GetMetadata and LoadContent with an
// error wrapping ErrFileTooLarge. A limit of 0 or less disables the
// check. In-memory documents (NewMDFileFromString, NewMDFileFromBytes)
// are already loaded and are not limited.
func WithMaxFileSize(limit int64) FileOption {
	return func(md *MDFile) {
		md.maxSize = limit
		md.maxSizeSet = true
	}
}

// sizeLimit returns the effective limit; 0 means unlimited.
func (md *MDFile) sizeLimit() int64 {
	if !md.maxSizeSet {
		return DefaultMaxFileSize
	}
	if md.maxSize <= 0 {
		return 0
	}
	return md.maxSize
}

// limitFile checks the size of an opened file against the limit and
// wraps it so reading past the limit fails too, which covers files
// that grow after the check and files whose size Stat cannot tell.
func (md *MDFile) limitFile(f io.ReadCloser) (io.ReadCloser, error) {
	limit := md.sizeLimit()
	if limit == 0 {
		return f, nil
	}
	if st, ok := f.(interface{ Stat() (fs.FileInfo, error) }); ok {
		if info, err := st.Stat(); err == nil && info.Mode().IsRegular() && info.Size() > limit {
			return nil, fileTooLarge(md.path, limit)
		}
	}
	return &limitedReadCloser{rc: f, left: limit, limit: limit, path: md.path}, nil
}

func fileTooLarge(path string, limit int64) error {
	return fmt.Errorf("%s: larger than %d bytes: %w", path, limit, ErrFileTooLarge)
}

// limitedReadCloser returns ErrFileTooLarge instead of io.EOF when the
// underlying reader has more than limit bytes. It reads at most one
// byte past the limit, so an oversized stream cannot exhaust memory.
type limitedReadCloser struct {
	rc    io.ReadCloser
	left  int64 // bytes still allowed
	limit int64
	path  string
}

func (l *limitedReadCloser) Read(p []byte) (int, error) {
	if l.left < 0 {
		return 0, fileTooLarge(l.path, l.limit)
	}
	if int64(len(p)) > l.left+1 {
		p = p[:l.left+1]
	}
	n, err := l.rc.Read(p)
	l.left -= int64(n)
	if l.left < 0 {
		// The byte past the limit proves the file is too large; do not
		// hand it to the caller.
		n--
		return n, fileTooLarge(l.path, l.limit)
	}
	return n, err
}

func (l *limitedReadCloser) Close() error { return l.rc.Close() }
