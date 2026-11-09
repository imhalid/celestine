package library

import (
	"os"
	"path"
	"path/filepath"
)

// SyncBases keep each page file as it was at the last sync, so that a page changed on both sides can be merged
// (MergePage). Only page files are kept, and none past maxBase: the rest is kept twice on a conflict, as before.
type SyncBases interface {
	Base(name string) []byte // nil when there is none
	SetBase(name string, data []byte)
	DropBase(name string)
}

// maxBase is the largest page file whose base is kept.
const maxBase = 1 << 20

// keepsBase says whether the file name has its base kept.
func keepsBase(name string) bool { return path.Base(name) == PageFileName }

// setBase keeps data as the base of name, when it is a file whose base is kept.
func (st *SyncState) setBase(name string, data []byte) {
	if st.Bases == nil || !keepsBase(name) {
		return
	}
	if len(data) > maxBase {
		st.Bases.DropBase(name)
		return
	}
	st.Bases.SetBase(name, data)
}

func (st *SyncState) dropBase(name string) {
	if st.Bases != nil && keepsBase(name) {
		st.Bases.DropBase(name)
	}
}

// MetaBases keeps the bases in a library file's meta, beside the sync's state.
type MetaBases struct{ Store *Store }

const baseMetaPrefix = "sync-base:"

func (m MetaBases) Base(name string) []byte          { return m.Store.Meta(baseMetaPrefix + name) }
func (m MetaBases) SetBase(name string, data []byte) { m.Store.SetMeta(baseMetaPrefix+name, data) }
func (m MetaBases) DropBase(name string)             { m.Store.DeleteMeta(baseMetaPrefix + name) }

// DirBases keeps the bases in a folder of their own, a file each, named by the hash of the file's name: a folder
// library's, in the config folder.
type DirBases struct{ Root string }

func (d DirBases) file(name string) string { return filepath.Join(d.Root, HashOf([]byte(name))) }

func (d DirBases) Base(name string) []byte {
	b, err := os.ReadFile(d.file(name))
	if err != nil {
		return nil
	}
	return b
}

func (d DirBases) SetBase(name string, data []byte) {
	if os.MkdirAll(d.Root, 0o700) == nil {
		WriteFilePrivate(d.file(name), data)
	}
}

func (d DirBases) DropBase(name string) { os.Remove(d.file(name)) }
