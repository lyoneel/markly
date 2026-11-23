//go:build !unix

package markly

import "os"

// keepOwner is a no-op where file ownership is not exposed.
func keepOwner(*os.File, os.FileInfo) {}
