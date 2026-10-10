package main

import (
	"embed"

	"github.com/egoist/mygo/ui"
)

// 图标采用 Tabler Icons 的独立 SVG 文件，并随 Go 二进制一起嵌入。
//
//go:embed resources/icons/*.svg
var embeddedIcons embed.FS

type iconSet struct {
	history   *ui.SVG
	devices   *ui.SVG
	settings  *ui.SVG
	search    *ui.SVG
	clear     *ui.SVG
	filter    *ui.SVG
	text      *ui.SVG
	link      *ui.SVG
	code      *ui.SVG
	image     *ui.SVG
	file      *ui.SVG
	copy      *ui.SVG
	check     *ui.SVG
	star      *ui.SVG
	chevron   *ui.SVG
	deviceAdd *ui.SVG
}

var icons = mustLoadIconSet()

func mustLoadIconSet() iconSet {
	return iconSet{
		history:   mustLoadIcon("history.svg"),
		devices:   mustLoadIcon("device-desktop.svg"),
		settings:  mustLoadIcon("settings.svg"),
		search:    mustLoadIcon("search.svg"),
		clear:     mustLoadIcon("x.svg"),
		filter:    mustLoadIcon("adjustments-horizontal.svg"),
		text:      mustLoadIcon("file-text.svg"),
		link:      mustLoadIcon("external-link.svg"),
		code:      mustLoadIcon("code.svg"),
		image:     mustLoadIcon("photo.svg"),
		file:      mustLoadIcon("file.svg"),
		copy:      mustLoadIcon("copy.svg"),
		check:     mustLoadIcon("check.svg"),
		star:      mustLoadIcon("star.svg"),
		chevron:   mustLoadIcon("chevron-down.svg"),
		deviceAdd: mustLoadIcon("device-plus.svg"),
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
	default:
		return icons.text
	}
}
