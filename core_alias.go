package main

// The library - its collections, pages, files, history and sync - is its own package (internal/library),
// which knows nothing of the window. These are its names as the app has always used them, unqualified; new
// code may as well write library.Page.

import (
	"github.com/imhalid/celestine/internal/library"

	"github.com/imhalid/celestine/locales"
)

type BodyFormat = library.BodyFormat

var Clean = library.Clean
var CleanOptions = library.CleanOptions

type Collection = library.Collection

var CompareManual = library.CompareManual
var CompareValues = library.CompareValues

const CollectionFile = library.CollectionFile

var CopyTree = library.CopyTree

const CoverNone = library.CoverNone

var DeriveKey = library.DeriveKey
var DiffLines = library.DiffLines

type DiffOp = library.DiffOp
type DirRemote = library.DirRemote
type EncRemote = library.EncRemote
type FS = library.FS

var FileSafe = library.FileSafe
var SplitTags = library.SplitTags

type Filter = library.Filter

var FilterOps = library.FilterOps
var HTTPSOnly = library.HTTPSOnly
var HashOf = library.HashOf
var InBatch = library.InBatch
var IsTempFile = library.IsTempFile

type Historian = library.Historian

const HistoryDays = library.HistoryDays
const IndexFile = library.IndexFile

var IsLibraryFolder = library.IsLibraryFolder
var JoinPage = library.JoinPage
var KeyFromRecovery = library.KeyFromRecovery
var LastOfDays = library.LastOfDays

const LegacyPageFile = library.LegacyPageFile

type Library = library.Library

var LibraryFile = library.LibraryFile
var ModTime = library.ModTime

type NamedTemplate = library.NamedTemplate
type NewerFormatError = library.NewerFormatError

var NewEncRemote = library.NewEncRemote
var NewHTTPRemote = library.NewHTTPRemote

type OSFS = library.OSFS

var OpenEnc = library.OpenEnc
var OpenLibrary = library.OpenLibrary
var OpenLibraryIn = library.OpenLibraryIn
var OpenStore = library.OpenStore
var OpenStoreReadOnly = library.OpenStoreReadOnly

type Page = library.Page

const PageFileName = library.PageFileName

var PageMeta = library.PageMeta
var PagesChanged = library.PagesChanged
var PairWords = library.PairWords
var Parents = library.Parents
var PristineOf = library.PristineOf

const PropChoice = library.PropChoice
const PropDate = library.PropDate
const PropNumber = library.PropNumber
const PropText = library.PropText

// PropTypeName is the name of a kind of property, in the window's language.
func PropTypeName(typ string) string {
	switch typ {
	case PropChoice:
		return locales.PropsTypeChoice()
	case PropText:
		return locales.PropsTypeText()
	case PropNumber:
		return locales.PropsTypeNumber()
	case PropDate:
		return locales.PropsTypeDate()
	case PropURL:
		return locales.PropsTypeURL()
	case PropPage:
		return locales.PropsTypePage()
	case PropTags:
		return locales.PropsTypeTags()
	case PropFile:
		return locales.PropsTypeFile()
	case PropAliases:
		return locales.PropsTypeAliases()
	}
	return typ
}

const PropURL = library.PropURL
const PropPage = library.PropPage
const PropTags = library.PropTags
const PropFile = library.PropFile
const PropAliases = library.PropAliases

// IsList reports whether a kind of property holds a list of words: tags and aliases.
func IsList(typ string) bool { return library.IsList(typ) }

type Property = library.Property

var ReadPages = library.ReadPages
var RecoveryKey = library.RecoveryKey

type Remote = library.Remote
type RemoteEntry = library.RemoteEntry
type RemoteVersion = library.RemoteVersion

var Renumber = library.Renumber
var SafeName = library.SafeName
var SafeSubpath = library.SafeSubpath

const SaltFile = library.SaltFile
const SettingsKey = library.SettingsKey

var Slug = library.Slug
var SortCollections = library.SortCollections
var SplitFrontMatter = library.SplitFrontMatter
var SplitPage = library.SplitPage
var SplitPageChecked = library.SplitPageChecked
var SplitTitle = library.SplitTitle

type Store = library.Store

const StoreRoot = library.StoreRoot

var SyncOnce = library.SyncOnce
var SyncCounted = library.SyncCounted

type SyncResult = library.SyncResult
type SyncState = library.SyncState
type MetaBases = library.MetaBases
type DirBases = library.DirBases
type ChangeCounter = library.ChangeCounter
type ExactCounter = library.ExactCounter

var TextOfBody = library.TextOfBody

const TrashDays = library.TrashDays
const TrashDir = library.TrashDir

type TrashItem = library.TrashItem

const TrashNote = library.TrashNote

var UniqueDir = library.UniqueDir

type Version = library.Version
type View = library.View

var Within = library.Within

type Word = library.Word

var WordHistory = library.WordHistory
var WriteFilePrivate = library.WriteFilePrivate

// The library's variables that the app reads: its counter itself, and the others, which do not change.
var (
	PagesGen    = &library.PagesGen
	SavesGen    = &library.SavesGen
	PropTypes   = library.PropTypes
	TurkishFold = library.TurkishFold
)
