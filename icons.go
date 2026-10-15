package main

import (
	"embed"
	"strings"

	"gopaste/internal/filetype"

	"github.com/egoist/mygo/ui"
)

// 图标采用 Tabler Icons 的独立 SVG 文件，并随 Go 二进制一起嵌入。
//
//go:embed resources/icons/*.svg
var embeddedIcons embed.FS

// 应用图标取自 resources/icon.png：mygo 打包时用它生成 macOS 的 .icns、Windows 的
// .ico 和 Linux 的 .png，托盘图标也用同一张，没有单独的托盘图标。
//
//go:embed resources/icon.png
var embeddedTrayIcon []byte

type iconSet struct {
	history      *ui.SVG
	list         *ui.SVG
	devices      *ui.SVG
	brandApple   *ui.SVG
	brandWindows *ui.SVG
	brandLinux   *ui.SVG
	brandUbuntu  *ui.SVG
	lan          *ui.SVG
	settings     *ui.SVG
	search       *ui.SVG
	clear        *ui.SVG
	refresh      *ui.SVG
	filter       *ui.SVG
	clock        *ui.SVG
	railCollapse *ui.SVG
	railExpand   *ui.SVG
	sun          *ui.SVG
	moon         *ui.SVG
	auto         *ui.SVG
	text         *ui.SVG
	richText     *ui.SVG
	link         *ui.SVG
	unlink       *ui.SVG
	code         *ui.SVG
	image        *ui.SVG
	file         *ui.SVG
	fileDoc      *ui.SVG
	fileSheet    *ui.SVG
	fileSlides   *ui.SVG
	filePDF      *ui.SVG
	fileCode     *ui.SVG
	spreadsheet  *ui.SVG
	csv          *ui.SVG
	markdown     *ui.SVG
	json         *ui.SVG
	config       *ui.SVG
	shell        *ui.SVG
	sql          *ui.SVG
	xml          *ui.SVG
	svg          *ui.SVG
	png          *ui.SVG
	jpg          *ui.SVG
	bmp          *ui.SVG
	archive      *ui.SVG
	audio        *ui.SVG
	video        *ui.SVG
	font         *ui.SVG
	executable   *ui.SVG
	brandGo      *ui.SVG
	brandPython  *ui.SVG
	brandJS      *ui.SVG
	brandTS      *ui.SVG
	brandVue     *ui.SVG
	brandSvelte  *ui.SVG
	brandRust    *ui.SVG
	brandPHP     *ui.SVG
	brandHTML    *ui.SVG
	brandCSS     *ui.SVG
	brandSass    *ui.SVG
	brandDocker  *ui.SVG
	brandFigma   *ui.SVG
	brandBlender *ui.SVG
	copy         *ui.SVG
	trash        *ui.SVG
	check        *ui.SVG
	star         *ui.SVG
	chevron      *ui.SVG
	deviceAdd    *ui.SVG
	appearance   *ui.SVG
	storage      *ui.SVG
	about        *ui.SVG
}

var icons = mustLoadIconSet()

func mustLoadIconSet() iconSet {
	return iconSet{
		history:      mustLoadIcon("history.svg"),
		list:         mustLoadIcon("list.svg"),
		devices:      mustLoadIcon("device-desktop.svg"),
		brandApple:   mustLoadIcon("brand-apple.svg"),
		brandWindows: mustLoadIcon("brand-windows.svg"),
		brandLinux:   mustLoadIcon("brand-linux.svg"),
		brandUbuntu:  mustLoadIcon("brand-ubuntu.svg"),
		lan:          mustLoadIcon("LAN.svg"),
		settings:     mustLoadIcon("settings.svg"),
		search:       mustLoadIcon("search.svg"),
		clear:        mustLoadIcon("x.svg"),
		refresh:      mustLoadIcon("refresh.svg"),
		filter:       mustLoadIcon("adjustments-horizontal.svg"),
		clock:        mustLoadIcon("clock.svg"),
		railCollapse: mustLoadIcon("layout-sidebar-left-collapse.svg"),
		railExpand:   mustLoadIcon("layout-sidebar-left-expand.svg"),
		sun:          mustLoadIcon("sun.svg"),
		moon:         mustLoadIcon("moon.svg"),
		auto:         mustLoadIcon("circle-half.svg"),
		text:         mustLoadIcon("file-text.svg"),
		richText:     mustLoadIcon("typography.svg"),
		link:         mustLoadIcon("external-link.svg"),
		unlink:       mustLoadIcon("unlink.svg"),
		code:         mustLoadIcon("code.svg"),
		image:        mustLoadIcon("photo.svg"),
		file:         mustLoadIcon("file.svg"),
		fileDoc:      mustLoadIcon("file-type-doc.svg"),
		fileSheet:    mustLoadIcon("file-type-xls.svg"),
		fileSlides:   mustLoadIcon("file-type-ppt.svg"),
		filePDF:      mustLoadIcon("file-type-pdf.svg"),
		fileCode:     mustLoadIcon("file-code.svg"),
		spreadsheet:  mustLoadIcon("file-spreadsheet.svg"),
		csv:          mustLoadIcon("file-type-csv.svg"),
		markdown:     mustLoadIcon("markdown.svg"),
		json:         mustLoadIcon("json.svg"),
		config:       mustLoadIcon("file-settings.svg"),
		shell:        mustLoadIcon("terminal-2.svg"),
		sql:          mustLoadIcon("file-type-sql.svg"),
		xml:          mustLoadIcon("file-type-xml.svg"),
		svg:          mustLoadIcon("file-type-svg.svg"),
		png:          mustLoadIcon("file-type-png.svg"),
		jpg:          mustLoadIcon("file-type-jpg.svg"),
		bmp:          mustLoadIcon("file-type-bmp.svg"),
		archive:      mustLoadIcon("file-type-zip.svg"),
		audio:        mustLoadIcon("file-music.svg"),
		video:        mustLoadIcon("movie.svg"),
		font:         mustLoadIcon("letter-case.svg"),
		executable:   mustLoadIcon("binary.svg"),
		brandGo:      mustLoadIcon("brand-golang.svg"),
		brandPython:  mustLoadIcon("brand-python.svg"),
		brandJS:      mustLoadIcon("brand-javascript.svg"),
		brandTS:      mustLoadIcon("brand-typescript.svg"),
		brandVue:     mustLoadIcon("brand-vue.svg"),
		brandSvelte:  mustLoadIcon("brand-svelte.svg"),
		brandRust:    mustLoadIcon("brand-rust.svg"),
		brandPHP:     mustLoadIcon("brand-php.svg"),
		brandHTML:    mustLoadIcon("brand-html5.svg"),
		brandCSS:     mustLoadIcon("brand-css3.svg"),
		brandSass:    mustLoadIcon("brand-sass.svg"),
		brandDocker:  mustLoadIcon("brand-docker.svg"),
		brandFigma:   mustLoadIcon("brand-figma.svg"),
		brandBlender: mustLoadIcon("brand-blender.svg"),
		copy:         mustLoadIcon("copy.svg"),
		trash:        mustLoadIcon("trash.svg"),
		check:        mustLoadIcon("check.svg"),
		star:         mustLoadIcon("star.svg"),
		chevron:      mustLoadIcon("chevron-down.svg"),
		deviceAdd:    mustLoadIcon("device-plus.svg"),
		appearance:   mustLoadIcon("palette.svg"),
		storage:      mustLoadIcon("database.svg"),
		about:        mustLoadIcon("info-circle.svg"),
	}
}

func mustLoadIcon(name string) *ui.SVG {
	data, err := embeddedIcons.ReadFile("resources/icons/" + name)
	if err != nil {
		panic("读取图标 " + name + " 失败: " + err.Error())
	}
	return ui.MustParseSVG(data)
}

func iconForKind(kind contentKind) *ui.SVG {
	switch kind {
	case kindLink:
		return icons.link
	case kindCode:
		return icons.code
	case kindImage:
		return icons.image
	case kindFile:
		return icons.file
	case kindRichText:
		return icons.richText
	default:
		return icons.text
	}
}

// iconForFormat picks the icon for a file payload's detected format, so the
// list and the preview show what a file is without the reader parsing its
// extension. An unrecognised format keeps the generic file icon.
func iconForFormat(format filetype.Format) *ui.SVG {
	switch format {
	case filetype.FormatText, filetype.FormatLog:
		return icons.text
	case filetype.FormatMarkdown:
		return icons.markdown
	case filetype.FormatRTF, filetype.FormatDocument:
		return icons.fileDoc
	case filetype.FormatSpreadsheet:
		return icons.spreadsheet
	case filetype.FormatXLS:
		return icons.fileSheet
	case filetype.FormatCSV:
		return icons.csv
	case filetype.FormatPresentation:
		return icons.fileSlides
	case filetype.FormatPDF:
		return icons.filePDF
	case filetype.FormatImage:
		return icons.image
	case filetype.FormatPNG:
		return icons.png
	case filetype.FormatJPEG:
		return icons.jpg
	case filetype.FormatBMP:
		return icons.bmp
	case filetype.FormatSVG:
		return icons.svg
	case filetype.FormatVideo:
		return icons.video
	case filetype.FormatAudio:
		return icons.audio
	case filetype.FormatArchive:
		return icons.archive
	case filetype.FormatCode:
		return icons.fileCode
	case filetype.FormatGo:
		return icons.brandGo
	case filetype.FormatPython:
		return icons.brandPython
	case filetype.FormatJavaScript:
		return icons.brandJS
	case filetype.FormatTypeScript:
		return icons.brandTS
	case filetype.FormatVue:
		return icons.brandVue
	case filetype.FormatSvelte:
		return icons.brandSvelte
	case filetype.FormatRust:
		return icons.brandRust
	case filetype.FormatPHP:
		return icons.brandPHP
	case filetype.FormatShell:
		return icons.shell
	case filetype.FormatSQL:
		return icons.sql
	case filetype.FormatHTML:
		return icons.brandHTML
	case filetype.FormatCSS:
		return icons.brandCSS
	case filetype.FormatSass:
		return icons.brandSass
	case filetype.FormatXML:
		return icons.xml
	case filetype.FormatJSON:
		return icons.json
	case filetype.FormatYAML:
		return icons.config
	case filetype.FormatConfig:
		return icons.settings
	case filetype.FormatDockerfile:
		return icons.brandDocker
	case filetype.FormatFont:
		return icons.font
	case filetype.FormatDatabase:
		return icons.storage
	case filetype.FormatDesign:
		return icons.appearance
	case filetype.FormatFigma:
		return icons.brandFigma
	case filetype.FormatBlender:
		return icons.brandBlender
	case filetype.FormatExecutable:
		return icons.executable
	default:
		return icons.file
	}
}

// iconForPlatform 根据设备上报的操作系统选择品牌图标。
// 平台名称大小写和写法可能不一致，这里做归一化匹配；
// 无法识别时回退到通用设备图标，避免出现空白占位。
func iconForPlatform(platform string) *ui.SVG {
	name := strings.ToLower(strings.TrimSpace(platform))
	switch {
	case strings.Contains(name, "mac"), strings.Contains(name, "darwin"), strings.Contains(name, "apple"):
		return icons.brandApple
	case strings.Contains(name, "windows"):
		return icons.brandWindows
	case strings.Contains(name, "ubuntu"):
		return icons.brandUbuntu
	case strings.Contains(name, "linux"):
		return icons.brandLinux
	default:
		return icons.devices
	}
}
