package validate

import (
	"fmt"
	"io"
	"io/fs"
	"os"

	"gitlab.com/lyoneel/markly"
)

// readFileLimited reads a document, schema or vocabulary file, refusing
// one larger than markly.DefaultMaxFileSize with an error wrapping
// markly.ErrFileTooLarge, the limit markly applies to the files it
// reads itself.
func readFileLimited(path string) ([]byte, error) {
	f, err := os.Open(path) // #nosec G304 -- the caller names the document or schema to validate
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return readLimited(f, info, path)
}

// readLimited reads at most one byte past the limit, so a file that
// grows while it is read cannot exhaust memory either.
func readLimited(r io.Reader, info fs.FileInfo, name string) ([]byte, error) {
	limit := markly.DefaultMaxFileSize
	if info.Mode().IsRegular() && info.Size() > limit {
		return nil, tooLarge(name, limit)
	}
	raw, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, tooLarge(name, limit)
	}
	return raw, nil
}

func tooLarge(name string, limit int64) error {
	return fmt.Errorf("%s: larger than %d bytes: %w", name, limit, markly.ErrFileTooLarge)
}
