// Package filetype classifies a copied file by its name so the interface can
// choose a matching icon, colour and label. It is pure: it never touches the
// filesystem, the clipboard or any UI type, so classification is cheap enough
// to run on every frame and is tested without a fixture on disk.
//
// Classification has two levels. A Format is the specific thing the file is —
// Markdown, JSON, Go, PNG — and earns its own icon. A Kind is the coarse
// family that format belongs to — text, code, image — and drives the label and
// the colour. Keeping them apart lets the table grow to many formats without
// the label and colour vocabulary growing with it.
package filetype

import "strings"

// Kind is the coarse family a file format belongs to. It is deliberately not a
// MIME type: the interface needs a handful of visually separable groups, not a
// complete registry.
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

// Format is a specific file format, fine-grained enough to earn its own icon.
type Format uint8

const (
	FormatUnknown Format = iota

	// Text and documents.
	FormatText
	FormatMarkdown
	FormatLog
	FormatRTF
	FormatDocument
	FormatSpreadsheet
	FormatXLS
	FormatCSV
	FormatPresentation
	FormatPDF

	// Media.
	FormatImage
	FormatPNG
	FormatJPEG
	FormatBMP
	FormatSVG
	FormatVideo
	FormatAudio

	// Archives and disk images.
	FormatArchive

	// Source and markup.
	FormatCode
	FormatGo
	FormatPython
	FormatJavaScript
	FormatTypeScript
	FormatVue
	FormatSvelte
	FormatRust
	FormatPHP
	FormatShell
	FormatSQL
	FormatHTML
	FormatCSS
	FormatSass
	FormatXML
	FormatJSON
	FormatYAML
	FormatConfig
	FormatDockerfile

	// Everything else.
	FormatFont
	FormatDatabase
	FormatDesign
	FormatFigma
	FormatBlender
	FormatExecutable
)

// Kind returns the coarse family a format belongs to.
func (f Format) Kind() Kind {
	switch f {
	case FormatText, FormatMarkdown, FormatLog:
		return KindText
	case FormatRTF, FormatDocument:
		return KindDocument
	case FormatSpreadsheet, FormatXLS, FormatCSV:
		return KindSpreadsheet
	case FormatPresentation:
		return KindPresentation
	case FormatPDF:
		return KindPDF
	case FormatImage, FormatPNG, FormatJPEG, FormatBMP, FormatSVG:
		return KindImage
	case FormatVideo:
		return KindVideo
	case FormatAudio:
		return KindAudio
	case FormatArchive:
		return KindArchive
	case FormatCode, FormatGo, FormatPython, FormatJavaScript, FormatTypeScript,
		FormatVue, FormatSvelte, FormatRust, FormatPHP, FormatShell, FormatSQL,
		FormatHTML, FormatCSS, FormatSass, FormatXML, FormatJSON, FormatYAML,
		FormatConfig, FormatDockerfile:
		return KindCode
	case FormatFont:
		return KindFont
	case FormatDatabase:
		return KindDatabase
	case FormatDesign, FormatFigma, FormatBlender:
		return KindDesign
	case FormatExecutable:
		return KindExecutable
	default:
		return KindUnknown
	}
}

// byExtension maps a lowercase extension without its dot to its format. The
// table is the whole implementation: one entry per extension the interface can
// name, and everything absent stays FormatUnknown.
var byExtension = map[string]Format{
	// Plain text.
	"txt": FormatText, "text": FormatText, "nfo": FormatText, "srt": FormatText,
	"vtt": FormatText, "rst": FormatText, "adoc": FormatText, "org": FormatText,
	"tex": FormatText,
	"log": FormatLog,
	"md":  FormatMarkdown, "markdown": FormatMarkdown, "mdx": FormatMarkdown,

	// Word-processor documents and e-books.
	"rtf": FormatRTF,
	"doc": FormatDocument, "docx": FormatDocument, "docm": FormatDocument,
	"odt": FormatDocument, "pages": FormatDocument, "epub": FormatDocument,
	"mobi": FormatDocument, "azw3": FormatDocument, "djvu": FormatDocument,

	// Spreadsheets.
	"csv": FormatCSV, "tsv": FormatCSV,
	"xls": FormatXLS, "xlsx": FormatXLS, "xlsm": FormatXLS,
	"ods": FormatSpreadsheet, "numbers": FormatSpreadsheet,

	// Slide decks.
	"ppt": FormatPresentation, "pptx": FormatPresentation, "pps": FormatPresentation,
	"ppsx": FormatPresentation, "odp": FormatPresentation, "key": FormatPresentation,

	// Portable documents.
	"pdf": FormatPDF,

	// Raster and vector images.
	"png": FormatPNG,
	"jpg": FormatJPEG, "jpeg": FormatJPEG,
	"bmp": FormatBMP,
	"svg": FormatSVG,
	"gif": FormatImage, "webp": FormatImage, "tif": FormatImage, "tiff": FormatImage,
	"heic": FormatImage, "heif": FormatImage, "avif": FormatImage, "ico": FormatImage,
	"icns": FormatImage, "raw": FormatImage, "cr2": FormatImage, "nef": FormatImage,
	"dng": FormatImage,

	// Video. "ts" stays code: a TypeScript file is far more likely on a
	// developer's clipboard than an MPEG transport stream, and the transport
	// stream keeps its explicit "mts" and "m2ts" names.
	"mp4": FormatVideo, "m4v": FormatVideo, "mov": FormatVideo, "avi": FormatVideo,
	"mkv": FormatVideo, "webm": FormatVideo, "wmv": FormatVideo, "flv": FormatVideo,
	"mpg": FormatVideo, "mpeg": FormatVideo, "3gp": FormatVideo, "mts": FormatVideo,
	"m2ts": FormatVideo,

	// Audio.
	"mp3": FormatAudio, "wav": FormatAudio, "flac": FormatAudio, "aac": FormatAudio,
	"ogg": FormatAudio, "oga": FormatAudio, "opus": FormatAudio, "m4a": FormatAudio,
	"wma": FormatAudio, "aiff": FormatAudio, "aif": FormatAudio, "mid": FormatAudio,
	"midi": FormatAudio, "amr": FormatAudio,

	// Archives and disk images.
	"zip": FormatArchive, "rar": FormatArchive, "7z": FormatArchive, "tar": FormatArchive,
	"gz": FormatArchive, "tgz": FormatArchive, "bz2": FormatArchive, "xz": FormatArchive,
	"zst": FormatArchive, "lz": FormatArchive, "lzma": FormatArchive, "cab": FormatArchive,
	"iso": FormatArchive, "dmg": FormatArchive, "jar": FormatArchive, "war": FormatArchive,

	// Languages and markup with an icon of their own.
	"go": FormatGo,
	"py": FormatPython, "pyw": FormatPython,
	"js": FormatJavaScript, "mjs": FormatJavaScript, "cjs": FormatJavaScript, "jsx": FormatJavaScript,
	"ts": FormatTypeScript, "tsx": FormatTypeScript,
	"vue":    FormatVue,
	"svelte": FormatSvelte,
	"rs":     FormatRust,
	"php":    FormatPHP,
	"sh":     FormatShell, "bash": FormatShell, "zsh": FormatShell, "fish": FormatShell,
	"ksh": FormatShell, "ps1": FormatShell, "psm1": FormatShell, "bat": FormatShell,
	"cmd":  FormatShell,
	"sql":  FormatSQL,
	"html": FormatHTML, "htm": FormatHTML, "xhtml": FormatHTML,
	"css":  FormatCSS,
	"scss": FormatSass, "sass": FormatSass, "less": FormatSass,
	"xml": FormatXML, "xsl": FormatXML, "xslt": FormatXML, "plist": FormatXML,
	"json": FormatJSON, "jsonc": FormatJSON, "json5": FormatJSON,
	"yaml": FormatYAML, "yml": FormatYAML,
	"toml": FormatConfig, "ini": FormatConfig, "cfg": FormatConfig, "conf": FormatConfig,
	"properties": FormatConfig, "env": FormatConfig,

	// Remaining source languages share the generic code icon.
	"rb": FormatCode, "java": FormatCode, "kt": FormatCode, "kts": FormatCode,
	"scala": FormatCode, "swift": FormatCode, "m": FormatCode, "mm": FormatCode,
	"c": FormatCode, "h": FormatCode, "cc": FormatCode, "cpp": FormatCode,
	"cxx": FormatCode, "hpp": FormatCode, "hh": FormatCode, "cs": FormatCode,
	"pl": FormatCode, "pm": FormatCode, "lua": FormatCode, "r": FormatCode,
	"dart": FormatCode, "ex": FormatCode, "exs": FormatCode, "erl": FormatCode,
	"hrl": FormatCode, "hs": FormatCode, "clj": FormatCode, "cljs": FormatCode,
	"fs": FormatCode, "fsx": FormatCode, "proto": FormatCode, "graphql": FormatCode,
	"gql": FormatCode, "tf": FormatCode, "hcl": FormatCode, "gradle": FormatCode,
	"cmake": FormatCode, "mk": FormatCode, "asm": FormatCode, "sol": FormatCode,
	"zig": FormatCode, "nim": FormatCode, "cr": FormatCode,

	// Fonts.
	"ttf": FormatFont, "otf": FormatFont, "ttc": FormatFont, "woff": FormatFont,
	"woff2": FormatFont, "eot": FormatFont, "pfb": FormatFont, "pfm": FormatFont,

	// Databases and dumps.
	"db": FormatDatabase, "db3": FormatDatabase, "sqlite": FormatDatabase,
	"sqlite3": FormatDatabase, "mdb": FormatDatabase, "accdb": FormatDatabase,
	"dbf": FormatDatabase, "dump": FormatDatabase,

	// Design and 3D assets.
	"fig":   FormatFigma,
	"blend": FormatBlender, "obj": FormatBlender, "fbx": FormatBlender,
	"stl": FormatBlender, "gltf": FormatBlender, "glb": FormatBlender,
	"3ds": FormatBlender, "dae": FormatBlender,
	"psd": FormatDesign, "psb": FormatDesign, "ai": FormatDesign, "eps": FormatDesign,
	"indd": FormatDesign, "idml": FormatDesign, "sketch": FormatDesign, "xd": FormatDesign,
	"afdesign": FormatDesign, "afphoto": FormatDesign, "afpub": FormatDesign,
	"dwg": FormatDesign, "dxf": FormatDesign, "cdr": FormatDesign, "xcf": FormatDesign,
	"kra": FormatDesign,

	// Executables and installers.
	"exe": FormatExecutable, "msi": FormatExecutable, "msp": FormatExecutable,
	"app": FormatExecutable, "apk": FormatExecutable, "ipa": FormatExecutable,
	"deb": FormatExecutable, "rpm": FormatExecutable, "pkg": FormatExecutable,
	"bin": FormatExecutable, "run": FormatExecutable, "dll": FormatExecutable,
	"so": FormatExecutable, "dylib": FormatExecutable, "com": FormatExecutable,
	"scr": FormatExecutable,
}

// byName maps an extension-less file name to its format. These are the
// well-known names whose whole identity is the name, such as Dockerfile.
var byName = map[string]Format{
	"dockerfile":  FormatDockerfile,
	"makefile":    FormatCode,
	"gnumakefile": FormatCode,
	"rakefile":    FormatCode,
	"gemfile":     FormatCode,
	"procfile":    FormatCode,
	"brewfile":    FormatCode,
	"vagrantfile": FormatCode,
	"justfile":    FormatCode,
	"license":     FormatText,
	"readme":      FormatText,
	"changelog":   FormatText,
}

// Detect classifies a file name by its extension, falling back to a few
// well-known extension-less names. The comparison ignores case, and a name
// with no extension or an unrecognised one is FormatUnknown.
//
// The name may be a full path; only its last element is classified.
func Detect(name string) Format {
	base := strings.ToLower(BaseName(name))
	if format, ok := byName[base]; ok {
		return format
	}
	extension := extensionOf(base)
	if extension == "" {
		return FormatUnknown
	}
	if format, ok := byExtension[extension]; ok {
		return format
	}
	return FormatUnknown
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
