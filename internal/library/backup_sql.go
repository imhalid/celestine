//go:build !js

package library

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
)

// Snapshot writes a consistent copy of the store to dst (a file that is not there), while the store goes on.
func (s *Store) Snapshot(dst string) error {
	db, err := sql.Open("sqlite", sqliteURI(s.File, "mode=ro&_pragma=busy_timeout(5000)")) // its own connection: the app's is not held
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(`VACUUM INTO '` + strings.ReplaceAll(filepath.ToSlash(dst), `'`, `''`) + `'`); err != nil {
		return err
	}
	return os.Chmod(dst, 0o600)
}
