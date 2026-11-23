package markly

import (
	"errors"

	"gitlab.com/lyoneel/dirly"
)

// SymlinkPolicy controls how MDFolder treats symlinks below the folder
// root. It is dirly's policy type, re-exported so callers do not need to
// import dirly.
type SymlinkPolicy = dirly.SymlinkPolicy

const (
	// FollowWithinRoot follows symlinks inside the folder, but refuses
	// files whose resolved target lands outside the resolved folder root.
	// Refused files are skipped and reported through MDFolder.Errors with
	// an error wrapping ErrPathEscapesRoot. This is the default.
	FollowWithinRoot = dirly.FollowWithinRoot

	// FollowAny follows symlinks wherever they point, including outside
	// the folder. This is the behavior of markly v1.7.1 and earlier.
	FollowAny = dirly.FollowAny

	// NoFollow refuses any file reached through a symlink below the
	// folder root. Refusals wrap ErrSymlinkNotAllowed.
	NoFollow = dirly.NoFollow
)

// DefaultSymlinkPolicy is the policy NewMDFolder uses unless
// WithSymlinkPolicy sets another one.
const DefaultSymlinkPolicy = FollowWithinRoot

var (
	// ErrPathEscapesRoot reports a folder file whose symlink target lies
	// outside the folder root. Detect it with errors.Is.
	ErrPathEscapesRoot = dirly.ErrPathEscapesRoot

	// ErrSymlinkNotAllowed reports a folder file reached through a
	// symlink under the NoFollow policy. Detect it with errors.Is.
	ErrSymlinkNotAllowed = dirly.ErrSymlinkNotAllowed

	// ErrInvalidSymlinkPolicy reports a SymlinkPolicy value that is not
	// FollowWithinRoot, FollowAny, or NoFollow.
	ErrInvalidSymlinkPolicy = errors.New("invalid symlink policy")
)

// validSymlinkPolicy reports whether p is one of the defined policies.
func validSymlinkPolicy(p SymlinkPolicy) bool {
	switch p {
	case FollowWithinRoot, FollowAny, NoFollow:
		return true
	}
	return false
}
