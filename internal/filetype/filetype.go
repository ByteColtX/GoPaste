// Package filetype classifies a copied file by its name so the interface can
// choose a matching icon and a preview strategy. It is pure: it never touches
// the filesystem, the clipboard or any UI type, so classification is cheap
// enough to run on every frame and is tested without a fixture on disk.
package filetype

import "strings"

// Kind is a coarse file category, chosen so each one earns a distinct icon and
// a distinct colour. It is deliberately not a MIME type: the interface needs a
// handful of visually separable groups, not a complete registry, and an
// unknown format is better left generic than guessed at.
type Kind uint8

const (
	// KindUnknown is a name with no extension, or one whose extension is not
	// recognised. The interface falls back to a generic file icon.
	KindUnknown Kind = iota
	KindText
	KindDocument
	KindSpreadsheet
	KindPresentation
	KindPDF
	KindImage
	KindVideo
	KindAudio
	KindArchive
	KindCode
	KindFont
	KindDatabase
	KindDesign
	KindExecutable
)

// byExtension maps a lowercase extension without its dot to its category.
// The table is the whole implementation: one entry per extension the interface
// can name, and everything absent stays KindUnknown.
var byExtension = map[string]Kind{
	// Plain text.
	"txt": KindText, "text": KindText, "log": KindText, "nfo": KindText,
	"md": KindText, "markdown": KindText, "rst": KindText, "adoc": KindText,
	"org": KindText, "tex": KindText, "srt": KindText, "vtt": KindText,
	"rtf": KindDocument, "csv": KindSpreadsheet, "tsv": KindSpreadsheet,

	// Word-processor and e-book documents.
	"doc": KindDocument, "docx": KindDocument, "docm": KindDocument,
	"odt": KindDocument, "pages": KindDocument, "epub": KindDocument,
	"mobi": KindDocument, "azw3": KindDocument, "djvu": KindDocument,

	// Spreadsheets.
	"xls": KindSpreadsheet, "xlsx": KindSpreadsheet, "xlsm": KindSpreadsheet,
	"ods": KindSpreadsheet, "numbers": KindSpreadsheet,

	// Slide decks.
	"ppt": KindPresentation, "pptx": KindPresentation, "pps": KindPresentation,
	"ppsx": KindPresentation, "odp": KindPresentation, "key": KindPresentation,

	// Portable documents.
	"pdf": KindPDF,

	// Raster and vector images.
	"png": KindImage, "jpg": KindImage, "jpeg": KindImage, "gif": KindImage,
	"webp": KindImage, "bmp": KindImage, "tif": KindImage, "tiff": KindImage,
	"heic": KindImage, "heif": KindImage, "avif": KindImage, "svg": KindImage,
	"ico": KindImage, "icns": KindImage, "raw": KindImage, "cr2": KindImage,
	"nef": KindImage, "dng": KindImage,

	// Video. "ts" stays code: a TypeScript file is far more likely on a
	// developer's clipboard than an MPEG transport stream, and the transport
	// stream keeps its explicit "mts" and "m2ts" names.
	"mp4": KindVideo, "m4v": KindVideo, "mov": KindVideo, "avi": KindVideo,
	"mkv": KindVideo, "webm": KindVideo, "wmv": KindVideo, "flv": KindVideo,
	"mpg": KindVideo, "mpeg": KindVideo, "3gp": KindVideo, "mts": KindVideo,
	"m2ts": KindVideo,

	// Audio.
	"mp3": KindAudio, "wav": KindAudio, "flac": KindAudio, "aac": KindAudio,
	"ogg": KindAudio, "oga": KindAudio, "opus": KindAudio, "m4a": KindAudio,
	"wma": KindAudio, "aiff": KindAudio, "aif": KindAudio, "mid": KindAudio,
	"midi": KindAudio, "amr": KindAudio,

	// Archives and disk images.
	"zip": KindArchive, "rar": KindArchive, "7z": KindArchive, "tar": KindArchive,
	"gz": KindArchive, "tgz": KindArchive, "bz2": KindArchive, "xz": KindArchive,
	"zst": KindArchive, "lz": KindArchive, "lzma": KindArchive, "cab": KindArchive,
	"iso": KindArchive, "dmg": KindArchive, "jar": KindArchive, "war": KindArchive,

	// Source and markup.
	"go": KindCode, "js": KindCode, "mjs": KindCode, "cjs": KindCode,
	"jsx": KindCode, "ts": KindCode, "tsx": KindCode, "vue": KindCode,
	"svelte": KindCode, "py": KindCode, "pyw": KindCode, "rb": KindCode,
	"rs": KindCode, "java": KindCode, "kt": KindCode, "kts": KindCode,
	"scala": KindCode, "swift": KindCode, "m": KindCode, "mm": KindCode,
	"c": KindCode, "h": KindCode, "cc": KindCode, "cpp": KindCode,
	"cxx": KindCode, "hpp": KindCode, "hh": KindCode, "cs": KindCode,
	"php": KindCode, "pl": KindCode, "pm": KindCode, "lua": KindCode,
	"r": KindCode, "dart": KindCode, "ex": KindCode, "exs": KindCode,
	"erl": KindCode, "hrl": KindCode, "hs": KindCode, "clj": KindCode,
	"cljs": KindCode, "fs": KindCode, "fsx": KindCode, "sh": KindCode,
	"bash": KindCode, "zsh": KindCode, "fish": KindCode, "ksh": KindCode,
	"ps1": KindCode, "psm1": KindCode, "bat": KindCode, "cmd": KindCode,
	"sql": KindCode, "html": KindCode, "htm": KindCode, "xhtml": KindCode,
	"css": KindCode, "scss": KindCode, "sass": KindCode, "less": KindCode,
	"json": KindCode, "jsonc": KindCode, "json5": KindCode, "yaml": KindCode,
	"yml": KindCode, "toml": KindCode, "ini": KindCode, "cfg": KindCode,
	"conf": KindCode, "properties": KindCode, "env": KindCode, "xml": KindCode,
	"xsl": KindCode, "xslt": KindCode, "plist": KindCode, "proto": KindCode,
	"graphql": KindCode, "gql": KindCode, "tf": KindCode, "hcl": KindCode,
	"gradle": KindCode, "cmake": KindCode, "mk": KindCode, "asm": KindCode,
	"sol": KindCode, "zig": KindCode, "nim": KindCode, "cr": KindCode,

	// Fonts.
	"ttf": KindFont, "otf": KindFont, "ttc": KindFont, "woff": KindFont,
	"woff2": KindFont, "eot": KindFont, "pfb": KindFont, "pfm": KindFont,

	// Databases and dumps.
	"db": KindDatabase, "db3": KindDatabase, "sqlite": KindDatabase,
	"sqlite3": KindDatabase, "mdb": KindDatabase, "accdb": KindDatabase,
	"dbf": KindDatabase, "dump": KindDatabase,

	// Design and 3D assets.
	"psd": KindDesign, "psb": KindDesign, "ai": KindDesign, "eps": KindDesign,
	"indd": KindDesign, "idml": KindDesign, "sketch": KindDesign, "fig": KindDesign,
	"xd": KindDesign, "afdesign": KindDesign, "afphoto": KindDesign,
	"afpub": KindDesign, "blend": KindDesign, "obj": KindDesign, "fbx": KindDesign,
	"stl": KindDesign, "gltf": KindDesign, "glb": KindDesign, "3ds": KindDesign,
	"dae": KindDesign, "dwg": KindDesign, "dxf": KindDesign, "cdr": KindDesign,
	"xcf": KindDesign, "kra": KindDesign,

	// Executables and installers.
	"exe": KindExecutable, "msi": KindExecutable, "msp": KindExecutable,
	"app": KindExecutable, "apk": KindExecutable, "ipa": KindExecutable,
	"deb": KindExecutable, "rpm": KindExecutable, "pkg": KindExecutable,
	"bin": KindExecutable, "run": KindExecutable, "dll": KindExecutable,
	"so": KindExecutable, "dylib": KindExecutable, "com": KindExecutable,
	"scr": KindExecutable,
}

// byName maps an extension-less file name to its category. These are the
// well-known names whose whole identity is the name, such as Dockerfile.
var byName = map[string]Kind{
	"dockerfile":  KindCode,
	"makefile":    KindCode,
	"gnumakefile": KindCode,
	"rakefile":    KindCode,
	"gemfile":     KindCode,
	"procfile":    KindCode,
	"brewfile":    KindCode,
	"vagrantfile": KindCode,
	"justfile":    KindCode,
	"license":     KindText,
	"readme":      KindText,
	"changelog":   KindText,
}

// Detect classifies a file name by its extension, falling back to a few
// well-known extension-less names. The comparison ignores case, and a name
// with no extension or an unrecognised one is KindUnknown.
//
// The name may be a full path; only its last element is classified.
func Detect(name string) Kind {
	base := strings.ToLower(BaseName(name))
	if kind, ok := byName[base]; ok {
		return kind
	}
	extension := extensionOf(base)
	if extension == "" {
		return KindUnknown
	}
	if kind, ok := byExtension[extension]; ok {
		return kind
	}
	return KindUnknown
}

// Extension returns a file name's lowercase extension without its dot, or ""
// when it has none. A leading dot names a hidden file rather than an
// extension, so ".gitignore" has none.
func Extension(name string) string {
	return extensionOf(strings.ToLower(BaseName(name)))
}

// BaseName returns the last element of a path, dropping any trailing
// separator. Both separators are accepted, because a path copied on Windows
// keeps its backslashes when it is shown on another platform.
func BaseName(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimRight(path, `/\`)
	if index := strings.LastIndexAny(path, `/\`); index >= 0 {
		return path[index+1:]
	}
	return path
}

// extensionOf splits the extension off an already-normalised base name.
func extensionOf(base string) string {
	dot := strings.LastIndexByte(base, '.')
	// A dot at index 0 is a hidden file, and a trailing dot is not an
	// extension either.
	if dot <= 0 || dot == len(base)-1 {
		return ""
	}
	return base[dot+1:]
}
