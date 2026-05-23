// Package files implements generic file tools missing from the standard library.
package files

import (
	"github.com/pkg/errors"
	"os"
	"os/user"
	"path"
	"strings"
)

// Exists returns true if file or directory exists.
func Exists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// ReplaceTildeInDir by the user's home directory. Returns dir if it doesn't start with "~".
//
// It returns an error if `dir` has an unknown user (e.g: `~unknown/...`)
func ReplaceTildeInDir(dir string) (string, error) {
	if len(dir) == 0 {
		return dir, nil
	}
	if dir[0] != '~' {
		return dir, nil
	}
	var userName string
	if dir != "~" && !strings.HasPrefix(dir, "~/") {
		sepIdx := strings.IndexRune(dir, '/')
		if sepIdx == -1 {
			userName = dir[1:]
		} else {
			userName = dir[1:sepIdx]
		}
	}
	var usr *user.User
	var err error
	if userName == "" {
		usr, err = user.Current()
	} else {
		usr, err = user.Lookup(userName)
	}
	if err != nil {
		return dir, errors.Wrapf(err, "failed to lookup home directory for user in path %q", dir)
	}
	homeDir := usr.HomeDir
	return path.Join(homeDir, dir[1+len(userName):]), nil
}
