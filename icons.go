package main

import (
	"embed"
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

// 图标采用 Tabler Icons 的独立 SVG 文件，并随 Go 二进制一起嵌入。
//
//go:embed resources/icons/*.svg
var embeddedIcons embed.FS

// 应用图标取自 resources/icon.png：mygo 打包时用它生成 macOS 的 .icns、Windows 的
// .ico 和 Linux 的 .png，这里也把它作为 Windows 和 Linux 的托盘图标嵌入。
//
//go:embed resources/icon.png
var embeddedTrayIcon []byte

// resources/tray-icon.png 是同一幅画的黑白模板版本：只有描边，背景透明。
// macOS 菜单栏图标只取 alpha 通道并按菜单栏明暗着色，整幅彩色图当模板用会被
// 涂成一整块实心方块，所以这里单独准备一张。
//
//go:embed resources/tray-icon.png
var embeddedTrayTemplateIcon []byte

// trayIcon 返回当前平台该用的托盘图标，以及它是否是 macOS 的模板图标。
func trayIcon() (png []byte, template bool) {
	if runtime.GOOS == "darwin" {
		return embeddedTrayTemplateIcon, true
	}
	return embeddedTrayIcon, false
}

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
