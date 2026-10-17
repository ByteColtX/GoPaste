// Command mockdb 生成 GoPaste 的本地预览数据库。
//
// 预览数据是剪贴板历史，属于敏感内容：本工具只在本机写入 -out 指定的数据库
// 文件，不联网、不上传，也不打印任何条目内容。数据库的结构、迁移、列编码和
// 投递记录全部由 internal/history/sqlite 负责，本工具不写任何 CREATE TABLE、
// PRAGMA 或原始 SQL，schema 因此只有一个权威来源。
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopaste/internal/history"
	"gopaste/internal/history/sqlite"
)

const (
	// defaultOutputPath 是默认输出文件。它故意不放在 resources/ 下：mygo dev
	// 递归监视那个目录，应用运行时的每次写入都会让它重建并重启；resources/ 里
	// 的东西也会被打进应用包。.preview 已被 .gitignore 忽略，两者都不沾。
	defaultOutputPath = ".preview/history.db"
	// defaultImagesDir 是默认的预览图片目录，里面的 PNG 就是预览数据用的图片。
	// 它和输出文件一样放在 .preview 下：resources/ 里的东西会被复制进应用包，
	// 这些照片只是走查素材，不能跟着产品分发。
	defaultImagesDir = ".preview/images"
	// previewIDPrefix 是每条预览记录 ID 的前缀，用来把它们和真实记录区分开。
	previewIDPrefix = "preview-"
)

// kindOrder 是摘要里的类型顺序，与 history.Kind 的取值一一对应。
var kindOrder = []history.Kind{
	history.KindText,
	history.KindLink,
	history.KindCode,
	history.KindImage,
	history.KindFile,
	history.KindRichText,
}

// kindLabels 是各类型在摘要里的中文名。
var kindLabels = map[history.Kind]string{
	history.KindText:     "文本",
	history.KindLink:     "链接",
	history.KindCode:     "代码",
	history.KindImage:    "图片",
	history.KindFile:     "文件",
	history.KindRichText: "富文本",
}

// previewDeviceIDs 是本地预览设备的稳定 ID，与 preview_devices.go 的
// previewPairedDevices 一一对应；投递记录按这个顺序写入。
var previewDeviceIDs = []string{
	"preview-mac",
	"preview-windows",
	"preview-imac-design",
	"preview-surface",
	"preview-thinkpad",
	"preview-ubuntu-workstation",
	"preview-mac-mini",
	"preview-windows-finance",
	"preview-fedora",
	"preview-macbook-air",
	"preview-windows-dev",
	"preview-ubuntu-laptop",
}

// onlineDeviceIDs 是预览设备里在线的那部分；其余设备按离线处理，用来构造
// 部分送达和离线待发两种同步状态。
var onlineDeviceIDs = map[string]bool{
	"preview-mac":         true,
	"preview-imac-design": true,
	"preview-surface":     true,
	"preview-thinkpad":    true,
	"preview-mac-mini":    true,
	"preview-fedora":      true,
	"preview-macbook-air": true,
}

// createdSpec 是一条预览记录的时间戳描述：要么取「今天已经过去的时间」的固定
// 比例，要么取「往前若干天」的某个时刻。所有时间戳都落在整分钟上，所以同一
// 分钟内重复运行会写出完全相同的行。
type createdSpec struct {
	today    bool
	fraction float64
	daysAgo  int
	hour     int
	minute   int
}

// todayAt 描述今天的一个时刻：fraction 是「今天已经过去的时间」的比例。
func todayAt(fraction float64) createdSpec {
	return createdSpec{today: true, fraction: fraction}
}

// daysAgoAt 描述往前 daysAgo 天的 hour:minute。
func daysAgoAt(daysAgo, hour, minute int) createdSpec {
	return createdSpec{daysAgo: daysAgo, hour: hour, minute: minute}
}

// fixture 是一条手写的预览记录；created 是相对时间描述，见 resolveCreated。
type fixture struct {
	id        string
	kind      history.Kind
	text      string
	html      string
	image     string
	fileSize  int64
	source    string
	created   createdSpec
	favorite  bool
	syncState history.SyncState
}

// buildFixture 返回全部预览记录。
//
// 时间戳只写相对描述：今天的记录按「今天已经过去的时间」取比例，保证任何时刻
// 运行都落在今天；其余记录写几天前的某个时刻。这样每个时间筛选都一定有内容。
func buildFixture() []fixture {
	return []fixture{
		// —— 今天 ——
		{
			id:        "preview-note-standup-today",
			kind:      history.KindText,
			text:      "站会记录\n昨天：完成设备列表的同步方向设置\n今天：接入图片类型的局域网传输\n风险：跨平台剪贴板格式差异",
			created:   todayAt(0.12),
			syncState: history.SyncStateSynced,
		},
		{
			id:        "preview-code-mygo-window",
			kind:      history.KindCode,
			text:      "mygo.App.WhenReady(func() {\n\tmygo.NewWindow(mygo.WindowOptions{\n\t\tTitle:   \"GoPaste\",\n\t\tContent: ui.View(app.view),\n\t})\n})",
			created:   todayAt(0.24),
			syncState: history.SyncStatePartial,
		},
		{
			id:      "preview-link-mygo-docs",
			kind:    history.KindLink,
			text:    "https://mygo.egoist.dev/docs/native-ui",
			source:  "preview-mac",
			created: todayAt(0.36),
		},
		{
			id:        "preview-image-aurora",
			kind:      history.KindImage,
			image:     "image-aurora.png",
			created:   todayAt(0.48),
			favorite:  true,
			syncState: history.SyncStateSynced,
		},
		{
			id:        "preview-rich-release-notes",
			kind:      history.KindRichText,
			text:      "发布说明 0.1.0\n- 局域网直连同步\n- 剪贴板历史筛选栏",
			html:      "<p>发布说明 <strong>0.1.0</strong></p>\n<ul><li>局域网直连同步</li><li>剪贴板历史筛选栏</li></ul>",
			created:   todayAt(0.60),
			favorite:  true,
			syncState: history.SyncStateSynced,
		},
		{
			id:       "preview-file-release-notes",
			kind:     history.KindFile,
			text:     "~/Documents/release-notes.md",
			fileSize: 3_482,
			created:  todayAt(0.72),
		},
		{
			id:        "preview-note-shortcuts",
			kind:      history.KindText,
			text:      "⌘⇧V：打开历史记录\n↑ / ↓：移动选择\n回车：复制当前记录\nEsc：关闭窗口",
			created:   todayAt(0.84),
			favorite:  true,
			syncState: history.SyncStatePartial,
		},
		{
			id:        "preview-code-pane-width",
			kind:      history.KindCode,
			text:      "const minHistoryPaneWidth = 300\nconst minPreviewPaneWidth = 460\n\nmaxHistoryWidth := windowWidth - railWidth - minPreviewPaneWidth - 1",
			created:   todayAt(0.94),
			syncState: history.SyncStateOffline,
		},
		{
			id:        "preview-note-long-spec",
			kind:      history.KindText,
			text:      "局域网同步设计要点\n\n1. 只允许已配对设备直连，不经过中继或云端。\n2. 配对时交换设备公钥，之后每条记录单独签名。\n3. 剪贴板内容只在内存里解密，落盘的是本地数据库。\n4. 设备离线时记录留在本机，等设备回到局域网再投递。\n5. 图片按 PNG 内联，文件只传路径与大小，正文留在原设备。\n6. 密码管理器标记过的内容不记录、不投递。\n\n待确认：\n- 单条记录的大小上限\n- 历史保留策略（按条数还是按天数）\n- 两端同时修改时以哪一端为准",
			created:   todayAt(0.16),
			favorite:  true,
			syncState: history.SyncStateSynced,
		},
		{
			id:      "preview-note-short",
			kind:    history.KindText,
			text:    "OK",
			created: todayAt(0.28),
		},
		{
			id:      "preview-note-long-line",
			kind:    history.KindText,
			text:    "这是一条刻意很长的单行文本，用来检查列表里的省略与预览区的换行：GoPaste 的历史列表在窄窗口下必须先把单行文本截断成一行加省略号，再在预览区按可用宽度重新折行，而不是把整行原样塞进列表让它横向溢出，也不是把行高撑开成两三行。",
			source:  "preview-mac",
			created: todayAt(0.34),
		},
		{
			id:        "preview-note-mixed-script",
			kind:      history.KindText,
			text:      "🎉 发布啦！GoPaste v0.1.0 — 局域网直连、无需账号、剪贴板历史留在本机。\nTry it: ⌘⇧V 打开历史，回车复制，Esc 关闭。",
			created:   todayAt(0.46),
			favorite:  true,
			syncState: history.SyncStatePartial,
		},
		{
			id:   "preview-code-json-payload",
			kind: history.KindCode,
			text: `{
  "device": { "id": "peer-9f2a", "name": "MacBook Pro", "online": true },
  "item": { "kind": "text", "size": 128, "createdAt": "2026-03-04T05:06:07Z" },
  "deliveries": [
    { "deviceId": "peer-1b7c", "state": "received" },
    { "deviceId": "peer-4d10", "state": "pending" }
  ]
}`,
			created:   todayAt(0.58),
			syncState: history.SyncStateSynced,
		},
		{
			id:      "preview-code-css-card",
			kind:    history.KindCode,
			text:    ".history-row {\n  display: flex;\n  align-items: center;\n  gap: 10px;\n  padding: 8px 12px;\n  border-radius: 8px;\n}\n\n.history-row[data-selected=\"true\"] {\n  background: color-mix(in srgb, currentColor 8%, transparent);\n}",
			source:  "preview-imac-design",
			created: todayAt(0.66),
		},
		{
			id:        "preview-code-diff-view",
			kind:      history.KindCode,
			text:      "--- a/view.go\n+++ b/view.go\n@@ -640,6 +640,7 @@ func (a *application) historyRow(c *ui.Context, item history.Item) {\n \tswitch item.Kind {\n \tcase kindCode:\n-\t\tui.Text(c, item.Text)\n+\t\tui.Text(c, item.Text).Font(ui.Monospace)\n \tdefault:\n \t\tui.Text(c, truncateSingleLine(item.Text, historyTextLimit))\n \t}",
			created:   todayAt(0.74),
			syncState: history.SyncStateOffline,
		},
		{
			id:        "preview-file-screenshots",
			kind:      history.KindFile,
			text:      "~/Desktop/截图 2026-03-04 05.06.07.png\n~/Desktop/截图 2026-03-04 05.06.11.png\n~/Desktop/截图 2026-03-04 05.06.14.png",
			fileSize:  2_734_080,
			created:   todayAt(0.82),
			favorite:  true,
			syncState: history.SyncStateSynced,
		},
		{
			id:      "preview-link-figma",
			kind:    history.KindLink,
			text:    "https://www.figma.com/design/8Qx2Kd/GoPaste?node-id=412-1187&t=Wq3n8Zb",
			created: todayAt(0.88),
		},
		{
			id:        "preview-rich-table",
			kind:      history.KindRichText,
			text:      "各平台支持情况\n平台\t剪贴板监听\t文件列表\nmacOS\t支持\t支持\nWindows\t支持\t支持\nLinux\t支持\t部分",
			html:      "<h3>各平台支持情况</h3>\n<table><thead><tr><th>平台</th><th>剪贴板监听</th><th>文件列表</th></tr></thead><tbody><tr><td>macOS</td><td>支持</td><td>支持</td></tr><tr><td>Windows</td><td>支持</td><td>支持</td></tr><tr><td>Linux</td><td>支持</td><td>部分</td></tr></tbody></table>",
			created:   todayAt(0.96),
			syncState: history.SyncStateSynced,
		},
		{
			id:      "preview-image-helix-today",
			kind:    history.KindImage,
			image:   "image-nebula-helix.png",
			source:  "preview-macbook-air",
			created: todayAt(0.99),
		},
		{
			id:        "preview-file-go-source",
			kind:      history.KindFile,
			text:      "~/Projects/GoPaste/internal/history/store.go",
			fileSize:  18_942,
			created:   todayAt(0.20),
			syncState: history.SyncStateSynced,
		},
		{
			id:       "preview-file-shell-script",
			kind:     history.KindFile,
			text:     "~/Projects/GoPaste/scripts/preview.sh",
			fileSize: 1_204,
			created:  todayAt(0.32),
		},
		{
			id:        "preview-file-dockerfile",
			kind:      history.KindFile,
			text:      "~/Projects/GoPaste/Dockerfile",
			fileSize:  612,
			created:   todayAt(0.44),
			syncState: history.SyncStatePartial,
		},
		{
			id:       "preview-file-installer-exe",
			kind:     history.KindFile,
			text:     "~/Downloads/GoPaste Setup 0.1.0.exe",
			fileSize: 31_457_280,
			created:  todayAt(0.56),
		},
		{
			id:        "preview-file-screen-recording",
			kind:      history.KindFile,
			text:      "~/Movies/录屏 2026-03-04.mov",
			fileSize:  184_549_376,
			created:   todayAt(0.68),
			syncState: history.SyncStateOffline,
		},
		// —— 昨天 ——
		{
			id:      "preview-image-grist-mill",
			kind:    history.KindImage,
			image:   "image-grist-mill.png",
			source:  "preview-mac",
			created: daysAgoAt(1, 7, 12),
		},
		{
			id:        "preview-image-canyon-de-chelly",
			kind:      history.KindImage,
			image:     "image-canyon-de-chelly.png",
			created:   daysAgoAt(1, 8, 8),
			syncState: history.SyncStateSynced,
		},
		{
			id:      "preview-code-clipboard-listener",
			kind:    history.KindCode,
			text:    "for event := range clipboardEvents {\n\titem := makeClipboardItem(event)\n\tstore.Append(item)\n\tstate.Select(item.ID)\n}",
			source:  "preview-mac",
			created: daysAgoAt(1, 9, 20),
		},
		{
			id:      "preview-link-layout-reference",
			kind:    history.KindLink,
			text:    "https://example.com/articles/clipboard-history-layout",
			source:  "preview-mac",
			created: daysAgoAt(1, 10, 42),
		},
		{
			id:        "preview-note-design-tasks",
			kind:      history.KindText,
			text:      "设计评审待办\n1. 收紧列表与分隔条的间距\n2. 补充空状态和滚动反馈\n3. 检查窄窗口下的最小宽度",
			created:   daysAgoAt(1, 11, 31),
			favorite:  true,
			syncState: history.SyncStateOffline,
		},
		{
			id:        "preview-rich-spec-document",
			kind:      history.KindRichText,
			text:      "GoPaste 同步协议草案\n1. 仅限局域网直连\n2. 配对后交换设备公钥\n3. 历史记录按条目投递",
			html:      "<h2>GoPaste 同步协议草案</h2>\n<ol><li>仅限局域网直连</li><li>配对后交换设备公钥</li><li>历史记录按条目投递</li></ol>",
			created:   daysAgoAt(1, 12, 5),
			favorite:  true,
			syncState: history.SyncStateSynced,
		},
		{
			id:       "preview-file-design-tokens",
			kind:     history.KindFile,
			text:     "~/Documents/GoPaste/design-tokens.json",
			fileSize: 12_640,
			source:   "preview-mac",
			created:  daysAgoAt(1, 15, 6),
		},
		{
			id:   "preview-code-html-card",
			kind: history.KindCode,
			text: `<article class="card">
  <h2 class="card-title">GoPaste</h2>
  <p class="card-body">剪贴板历史留在本机。</p>
  <a class="card-link" href="/download">下载</a>
</article>`,
			created: daysAgoAt(1, 16, 22),
		},
		{
			id:      "preview-code-c-parse",
			kind:    history.KindCode,
			text:    "static size_t read_len(const char *buf, size_t cap, size_t *out) {\n    if (buf == NULL || out == NULL) {\n        return EINVAL;\n    }\n    *out = cap > 4096 ? 4096 : cap;\n    return 0;\n}",
			source:  "preview-fedora",
			created: daysAgoAt(1, 17, 40),
		},
		{
			id:        "preview-code-swift-decode",
			kind:      history.KindCode,
			text:      "struct Device: Decodable {\n    let id: String\n    let name: String\n    let online: Bool\n}\n\nlet devices = try JSONDecoder().decode([Device].self, from: data)\nlet online = devices.filter { $0.online }",
			created:   daysAgoAt(1, 19, 5),
			favorite:  true,
			syncState: history.SyncStatePartial,
		},
		{
			id:      "preview-code-protobuf",
			kind:    history.KindCode,
			text:    "message ClipboardItem {\n  string id = 1;\n  Kind kind = 2;\n  bytes payload = 3;\n  int64 created_at_ms = 4;\n  repeated Delivery deliveries = 5;\n}\n\nenum Kind {\n  KIND_UNSPECIFIED = 0;\n  KIND_TEXT = 1;\n  KIND_IMAGE = 2;\n}",
			source:  "preview-surface",
			created: daysAgoAt(1, 20, 18),
		},
		{
			id:      "preview-link-stackoverflow",
			kind:    history.KindLink,
			text:    "https://stackoverflow.com/questions/12345678/how-to-watch-the-system-clipboard-on-macos-without-polling",
			created: daysAgoAt(1, 21, 9),
		},
		{
			id:      "preview-link-localhost",
			kind:    history.KindLink,
			text:    "http://localhost:5174/#/devices?filter=online&sort=name",
			created: daysAgoAt(1, 22, 30),
		},
		{
			id:        "preview-rich-blockquote",
			kind:      history.KindRichText,
			text:      "评审意见\n历史列表的密度可以再紧一点。\n\n> 引自设计评审：一屏能看到 8 条以上才算合格。\n\n参考实现：\nrowHeight := 82 - 6",
			html:      "<p>评审意见</p><p>历史列表的密度可以再紧一点。</p>\n<blockquote>引自设计评审：一屏能看到 8 条以上才算合格。</blockquote>\n<p>参考实现：</p><pre><code>rowHeight := 82 - 6</code></pre>",
			created:   daysAgoAt(1, 23, 12),
			syncState: history.SyncStateSynced,
		},
		{
			id:        "preview-file-pdf",
			kind:      history.KindFile,
			text:      "~/Documents/GoPaste/同步协议 v3.pdf",
			fileSize:  4_718_592,
			created:   daysAgoAt(1, 23, 48),
			syncState: history.SyncStateSynced,
		},
		{
			id:        "preview-note-error-log",
			kind:      history.KindText,
			text:      "2026-03-04T05:06:07Z WARN  device peer-4d10 offline, retry in 30s\n2026-03-04T05:06:37Z WARN  device peer-4d10 offline, retry in 30s\n2026-03-04T05:07:07Z ERROR dial 192.168.1.95:7710: connection refused\n2026-03-04T05:07:37Z INFO  delivery paused for peer-4d10",
			created:   daysAgoAt(1, 23, 59),
			syncState: history.SyncStateOffline,
		},
		{
			id:      "preview-image-canyon-yesterday",
			kind:    history.KindImage,
			image:   "image-canyon-de-chelly.png",
			source:  "preview-windows",
			created: daysAgoAt(1, 6, 40),
		},
		{
			id:       "preview-file-contract-docx",
			kind:     history.KindFile,
			text:     "~/Documents/合同模板.docx",
			fileSize: 46_080,
			created:  daysAgoAt(1, 13, 5),
		},
		{
			id:        "preview-file-quarterly-xlsx",
			kind:      history.KindFile,
			text:      "~/Downloads/季度报表.xlsx",
			fileSize:  318_464,
			created:   daysAgoAt(1, 13, 40),
			syncState: history.SyncStateSynced,
		},
		{
			id:       "preview-file-photo-jpg",
			kind:     history.KindFile,
			text:     "~/Pictures/IMG_2043.jpg",
			fileSize: 3_874_816,
			source:   "preview-imac-design",
			created:  daysAgoAt(1, 14, 12),
		},
		{
			id:        "preview-file-api-ts",
			kind:      history.KindFile,
			text:      "~/Projects/web/api.ts",
			fileSize:  4_208,
			created:   daysAgoAt(1, 14, 55),
			syncState: history.SyncStatePartial,
		},
		{
			id:       "preview-file-font-ttf",
			kind:     history.KindFile,
			text:     "~/Library/Fonts/Inter-Variable.ttf",
			fileSize: 812_544,
			source:   "preview-imac-design",
			created:  daysAgoAt(1, 18, 20),
		},
		{
			id:        "preview-file-history-db",
			kind:      history.KindFile,
			text:      "~/Library/Application Support/GoPaste/history.db",
			fileSize:  1_048_576,
			created:   daysAgoAt(1, 18, 48),
			syncState: history.SyncStateOffline,
		},
		// —— 最近 7 天 ——
		{
			id:        "preview-code-typescript-fetch",
			kind:      history.KindCode,
			text:      "export async function fetchHistory(page: number): Promise<Item[]> {\n  const res = await fetch(`/api/history?page=${page}`)\n  if (!res.ok) throw new Error(`HTTP ${res.status}`)\n  return res.json()\n}",
			created:   daysAgoAt(2, 10, 5),
			syncState: history.SyncStatePartial,
		},
		{
			id:        "preview-note-meeting-notes",
			kind:      history.KindText,
			text:      "周一同步会\n确认历史列表、设备页和快捷键的首版范围。\n\n下一步：接入本地剪贴板监听，再补充设备发现状态。",
			created:   daysAgoAt(2, 15, 54),
			syncState: history.SyncStatePartial,
		},
		{
			id:      "preview-note-mac-mini-meeting-room",
			kind:    history.KindText,
			text:    "会议室投屏信息\nWi-Fi：GoPaste-Guest\n投屏地址：http://192.168.1.88:8080",
			source:  "preview-mac-mini",
			created: daysAgoAt(2, 9, 26),
		},
		{
			id:      "preview-code-windows-build",
			kind:    history.KindCode,
			text:    "SET CGO_ENABLED=0\ngo tool mygo build -platform windows/amd64",
			source:  "preview-windows-dev",
			created: daysAgoAt(4, 16, 48),
		},
		{
			id:        "preview-file-csv-export",
			kind:      history.KindFile,
			text:      "~/Downloads/clipboard-history.csv",
			fileSize:  246_912,
			created:   daysAgoAt(2, 18, 16),
			syncState: history.SyncStateSynced,
		},
		{
			id:      "preview-image-cormorants",
			kind:    history.KindImage,
			image:   "image-cormorants.png",
			source:  "preview-windows",
			created: daysAgoAt(2, 19, 30),
		},
		{
			id:      "preview-code-python-intervals",
			kind:    history.KindCode,
			text:    "def merge_intervals(intervals):\n    intervals.sort(key=lambda x: x[0])\n    merged = []\n    for start, end in intervals:\n        if merged and start <= merged[-1][1]:\n            merged[-1][1] = max(merged[-1][1], end)\n        else:\n            merged.append([start, end])\n    return merged",
			source:  "preview-mac",
			created: daysAgoAt(2, 21, 40),
		},
		{
			id:       "preview-code-rust-prefix",
			kind:     history.KindCode,
			text:     "fn longest_common_prefix(words: &[String]) -> String {\n    let mut prefix = words.first().cloned().unwrap_or_default();\n    for word in &words[1..] {\n        while !word.starts_with(&prefix) {\n            prefix.pop();\n        }\n    }\n    prefix\n}",
			created:  daysAgoAt(3, 11, 18),
			favorite: true,
		},
		{
			id:      "preview-link-apple-hig",
			kind:    history.KindLink,
			text:    "https://developer.apple.com/design/human-interface-guidelines/",
			source:  "preview-mac",
			created: daysAgoAt(3, 14, 26),
		},
		{
			id:        "preview-image-danxia",
			kind:      history.KindImage,
			image:     "image-danxia.png",
			created:   daysAgoAt(3, 15, 50),
			favorite:  true,
			syncState: history.SyncStatePartial,
		},
		{
			id:        "preview-link-tabler-icons",
			kind:      history.KindLink,
			text:      "https://tabler.io/icons",
			created:   daysAgoAt(3, 16, 8),
			favorite:  true,
			syncState: history.SyncStateReplaced,
		},
		{
			id:      "preview-code-shell-gzip",
			kind:    history.KindCode,
			text:    "#!/usr/bin/env bash\nset -euo pipefail\n\nfor file in \"$@\"; do\n  gzip -9 \"$file\"\ndone",
			source:  "preview-fedora",
			created: daysAgoAt(4, 8, 55),
		},
		{
			id:        "preview-code-search-filter",
			kind:      history.KindCode,
			text:      "query := strings.ToLower(strings.TrimSpace(search))\ntext := strings.ToLower(strings.Join([]string{title, preview, content, source}, \" \"))\nreturn strings.Contains(text, query)",
			created:   daysAgoAt(4, 18, 37),
			syncState: history.SyncStatePartial,
		},
		{
			id:      "preview-code-device-handshake",
			kind:    history.KindCode,
			text:    "peer, err := lan.Dial(ctx, device.Address)\nif err != nil {\n\treturn fmt.Errorf(\"局域网直连失败: %w\", err)\n}\ndefer peer.Close()",
			source:  "preview-ubuntu-workstation",
			created: daysAgoAt(4, 20, 12),
		},
		{
			id:      "preview-image-octopus",
			kind:    history.KindImage,
			image:   "image-octopus.png",
			source:  "preview-imac-design",
			created: daysAgoAt(4, 21, 5),
		},
		{
			id:        "preview-code-yaml-ci",
			kind:      history.KindCode,
			text:      "name: build\non:\n  push:\n    branches: [main]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - run: go test ./...",
			created:   daysAgoAt(5, 9, 12),
			syncState: history.SyncStateSynced,
		},
		{
			id:      "preview-image-penguins",
			kind:    history.KindImage,
			image:   "image-penguins.png",
			source:  "preview-thinkpad",
			created: daysAgoAt(5, 10, 18),
		},
		{
			id:      "preview-code-sql-report",
			kind:    history.KindCode,
			text:    "SELECT device_id, COUNT(*) AS items\nFROM clipboard_history\nWHERE created_at >= now() - interval '7 days'\nGROUP BY device_id\nORDER BY items DESC\nLIMIT 10;",
			source:  "preview-windows-finance",
			created: daysAgoAt(5, 14, 30),
		},
		{
			id:        "preview-link-github-mygo",
			kind:      history.KindLink,
			text:      "https://github.com/egoist/mygo",
			created:   daysAgoAt(5, 20, 16),
			syncState: history.SyncStateSynced,
		},
		{
			id:        "preview-code-dockerfile",
			kind:      history.KindCode,
			text:      "FROM golang:1.23-alpine AS build\nWORKDIR /src\nCOPY . .\nRUN CGO_ENABLED=0 go build -o /out/gopaste .\n\nFROM gcr.io/distroless/static\nCOPY --from=build /out/gopaste /gopaste\nENTRYPOINT [\"/gopaste\"]",
			created:   daysAgoAt(6, 9, 48),
			syncState: history.SyncStatePartial,
		},
		{
			id:      "preview-code-toml-config",
			kind:    history.KindCode,
			text:    "[server]\nhost = \"127.0.0.1\"\nport = 7710\n\n[server.tls]\nenabled = true\ncert = \"certs/local.pem\"",
			source:  "preview-mac",
			created: daysAgoAt(6, 13, 22),
		},
		{
			id:        "preview-note-release-checklist",
			kind:      history.KindText,
			text:      "项目发布检查清单\n- 构建产物可启动\n- 图标资源保留 MIT 许可\n- 窗口尺寸和分栏可调整\n- 首次启动有合理的空状态",
			created:   daysAgoAt(6, 15, 41),
			syncState: history.SyncStateSynced,
		},
		{
			id:        "preview-image-nebula-trifid",
			kind:      history.KindImage,
			image:     "image-nebula-trifid.png",
			created:   daysAgoAt(6, 15, 50),
			favorite:  true,
			syncState: history.SyncStateSynced,
		},
		{
			id:       "preview-file-export-txt",
			kind:     history.KindFile,
			text:     "~/Desktop/clipboard-export.txt",
			fileSize: 1_286_144,
			source:   "preview-mac",
			created:  daysAgoAt(6, 16, 47),
			favorite: true,
		},
		{
			id:      "preview-code-kotlin",
			kind:    history.KindCode,
			text:    "fun devicesByPlatform(devices: List<Device>): Map<String, List<Device>> =\n    devices.filter { it.paired }.groupBy { it.platform }",
			source:  "preview-thinkpad",
			created: daysAgoAt(6, 18, 3),
		},
		{
			id:      "preview-code-lua",
			kind:    history.KindCode,
			text:    "local function trim(s)\n  return (s:gsub(\"^%s*(.-)%s*$\", \"%1\"))\nend\n\nfor _, path in ipairs(paths) do\n  print(trim(path))\nend",
			created: daysAgoAt(6, 19, 21),
		},
		{
			id:        "preview-code-zig",
			kind:      history.KindCode,
			text:      "pub fn digest(bytes: []const u8, out: *[32]u8) void {\n    var hasher = std.crypto.hash.sha2.Sha256.init(.{});\n    hasher.update(bytes);\n    hasher.final(out);\n}",
			created:   daysAgoAt(6, 20, 44),
			syncState: history.SyncStatePartial,
		},
		{
			id:      "preview-code-graphql",
			kind:    history.KindCode,
			text:    "query History($first: Int!, $kind: Kind) {\n  history(first: $first, kind: $kind) {\n    edges {\n      node {\n        id\n        kind\n        createdAt\n        source { id name }\n      }\n    }\n  }\n}",
			source:  "preview-mac",
			created: daysAgoAt(5, 9, 55),
		},
		{
			id:        "preview-code-terraform",
			kind:      history.KindCode,
			text:      "resource \"aws_instance\" \"relay\" {\n  ami           = var.ami\n  instance_type = \"t4g.small\"\n  tags = {\n    Name = \"gopaste-relay\"\n  }\n}",
			created:   daysAgoAt(5, 11, 17),
			syncState: history.SyncStateSynced,
		},
		{
			id:      "preview-code-nginx",
			kind:    history.KindCode,
			text:    "server {\n    listen 443 ssl http2;\n    server_name gopaste.local;\n\n    location / {\n        proxy_pass http://127.0.0.1:7710;\n        proxy_set_header Host $host;\n    }\n}",
			source:  "preview-ubuntu-workstation",
			created: daysAgoAt(5, 13, 2),
		},
		{
			id:      "preview-link-github-pr",
			kind:    history.KindLink,
			text:    "https://github.com/egoist/mygo/pull/812/files#diff-2f9a1c8b",
			created: daysAgoAt(5, 16, 38),
		},
		{
			id:        "preview-link-youtube",
			kind:      history.KindLink,
			text:      "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=1m42s",
			created:   daysAgoAt(5, 17, 55),
			favorite:  true,
			syncState: history.SyncStateReplaced,
		},
		{
			id:      "preview-link-arxiv",
			kind:    history.KindLink,
			text:    "https://arxiv.org/abs/2401.01234",
			source:  "preview-mac",
			created: daysAgoAt(4, 10, 26),
		},
		{
			id:        "preview-file-folder-copy",
			kind:      history.KindFile,
			text:      folderCopyPaths(),
			fileSize:  4_918_016,
			created:   daysAgoAt(4, 14, 9),
			favorite:  true,
			syncState: history.SyncStateSynced,
		},
		{
			id:      "preview-rich-email",
			kind:    history.KindRichText,
			text:    "主题：局域网同步的下一步\n你好，\n\n历史记录已经改成本地 SQLite，下一步是设备发现。\n先只做局域网广播，不做云端回退。\n\n祝好",
			html:    "<p><strong>主题：</strong>局域网同步的下一步</p><p>你好，</p><p>历史记录已经改成本地 SQLite，下一步是设备发现。<br>先只做局域网广播，<em>不做云端回退</em>。</p><p>祝好</p>",
			source:  "preview-windows",
			created: daysAgoAt(4, 17, 33),
		},
		{
			id:        "preview-note-shell-history",
			kind:      history.KindText,
			text:      "$ go tool mygo vet .\n$ CGO_ENABLED=0 go test ./...\nok  \tgopaste\t2.6s\nok  \tgopaste/internal/history\t0.1s\n$ git push origin main",
			created:   daysAgoAt(3, 20, 50),
			syncState: history.SyncStatePartial,
		},
		{
			id:      "preview-image-penguins-week",
			kind:    history.KindImage,
			image:   "image-penguins.png",
			created: daysAgoAt(2, 12, 14),
		},
		{
			id:       "preview-file-budget-numbers",
			kind:     history.KindFile,
			text:     "~/Documents/预算表.numbers",
			fileSize: 268_288,
			source:   "preview-windows-finance",
			created:  daysAgoAt(2, 13, 26),
		},
		{
			id:       "preview-file-animation-gif",
			kind:     history.KindFile,
			text:     "~/Pictures/动效预览.gif",
			fileSize: 5_242_880,
			created:  daysAgoAt(2, 16, 4),
		},
		{
			id:       "preview-file-legacy-bmp",
			kind:     history.KindFile,
			text:     "~/Pictures/老截图.bmp",
			fileSize: 6_291_456,
			source:   "preview-windows",
			created:  daysAgoAt(2, 17, 41),
		},
		{
			id:        "preview-file-icons-svg",
			kind:      history.KindFile,
			text:      "~/Documents/Design/图标.svg",
			fileSize:  9_318,
			created:   daysAgoAt(3, 9, 12),
			syncState: history.SyncStateSynced,
		},
		{
			id:       "preview-file-voice-memo",
			kind:     history.KindFile,
			text:     "~/Music/语音备忘 05.m4a",
			fileSize: 2_097_152,
			source:   "preview-macbook-air",
			created:  daysAgoAt(3, 12, 33),
		},
		{
			id:        "preview-file-python-tool",
			kind:      history.KindFile,
			text:      "~/Projects/tools/merge_intervals.py",
			fileSize:  2_846,
			created:   daysAgoAt(3, 17, 8),
			syncState: history.SyncStatePartial,
		},
		{
			id:       "preview-file-web-js",
			kind:     history.KindFile,
			text:     "~/Projects/web/history.js",
			fileSize: 7_412,
			source:   "preview-fedora",
			created:  daysAgoAt(4, 11, 52),
		},
		{
			id:       "preview-file-vue-component",
			kind:     history.KindFile,
			text:     "~/Projects/web/HistoryList.vue",
			fileSize: 5_120,
			created:  daysAgoAt(4, 16, 19),
		},
		{
			id:        "preview-file-ci-yaml",
			kind:      history.KindFile,
			text:      "~/Projects/GoPaste/.github/workflows/build.yaml",
			fileSize:  1_736,
			created:   daysAgoAt(5, 10, 7),
			syncState: history.SyncStateSynced,
		},
		{
			id:       "preview-file-figma-design",
			kind:     history.KindFile,
			text:     "~/Documents/Design/GoPaste.fig",
			fileSize: 12_582_912,
			source:   "preview-imac-design",
			created:  daysAgoAt(6, 14, 28),
		},
		// —— 最近 30 天 ——
		{
			id:      "preview-image-nebula-helix",
			kind:    history.KindImage,
			image:   "image-nebula-helix.png",
			source:  "preview-fedora",
			created: daysAgoAt(8, 21, 12),
		},
		{
			id:        "preview-note-onboarding",
			kind:      history.KindText,
			text:      "在同一局域网内打开 GoPaste，即可开始配对。\n\n配对成功后，新的剪贴板记录会在设备之间实时同步。",
			created:   daysAgoAt(12, 13, 22),
			syncState: history.SyncStateSynced,
		},
		{
			id:       "preview-file-design-export",
			kind:     history.KindFile,
			text:     "~/Desktop/design-export.zip",
			fileSize: 18_874_368,
			source:   "preview-imac-design",
			created:  daysAgoAt(14, 14, 2),
			favorite: true,
		},
		{
			id:        "preview-code-dotenv",
			kind:      history.KindCode,
			text:      "# GoPaste local config\nGOPASTE_PORT=7710\nGOPASTE_LOG_LEVEL=info\nGOPASTE_LAN_ONLY=true",
			created:   daysAgoAt(16, 10, 27),
			favorite:  true,
			syncState: history.SyncStateOffline,
		},
		{
			id:      "preview-rich-newsletter",
			kind:    history.KindRichText,
			text:    "本周版本说明\n新增：局域网设备发现\n修复：历史列表滚动抖动\n已知问题：Windows 下图片预览偶发空白",
			html:    "<h3>本周版本说明</h3>\n<p><strong>新增：</strong>局域网设备发现</p>\n<p><strong>修复：</strong>历史列表滚动抖动</p>\n<p><strong>已知问题：</strong>Windows 下图片预览偶发空白</p>",
			source:  "preview-windows",
			created: daysAgoAt(18, 9, 40),
		},
		{
			id:      "preview-image-earth-iss",
			kind:    history.KindImage,
			image:   "image-earth-iss.png",
			source:  "preview-macbook-air",
			created: daysAgoAt(20, 9, 5),
		},
		{
			id:      "preview-code-makefile",
			kind:    history.KindCode,
			text:    ".PHONY: build test fmt\n\nbuild:\n\tgo build -o build/gopaste .\n\ntest:\n\tgo test ./...\n\nfmt:\n\tgofmt -w .",
			source:  "preview-ubuntu-workstation",
			created: daysAgoAt(21, 12, 15),
		},
		{
			id:      "preview-link-mygo-github",
			kind:    history.KindLink,
			text:    "https://github.com/egoist/mygo/releases",
			source:  "preview-mac",
			created: daysAgoAt(24, 20, 16),
		},
		{
			id:        "preview-note-lan-only",
			kind:      history.KindText,
			text:      "同步只走局域网直连：不经过中继、云端或第三方代理。\n\n配对设备离线时，记录留在本机等待投递。",
			created:   daysAgoAt(27, 17, 5),
			syncState: history.SyncStatePartial,
		},
		{
			id:      "preview-code-ruby",
			kind:    history.KindCode,
			text:    "devices.group_by(&:platform).transform_values do |list|\n  list.sort_by(&:name).map(&:display_name)\nend",
			source:  "preview-thinkpad",
			created: daysAgoAt(26, 9, 31),
		},
		{
			id:      "preview-code-php",
			kind:    history.KindCode,
			text:    "$online = array_filter($devices, fn ($d) => $d['online']);\nusort($online, fn ($a, $b) => strcmp($a['name'], $b['name']));\necho count($online), \" 台在线\";",
			created: daysAgoAt(25, 15, 8),
		},
		{
			id:        "preview-code-sql-migration",
			kind:      history.KindCode,
			text:      "BEGIN;\nALTER TABLE history_items ADD COLUMN html TEXT NOT NULL DEFAULT '';\nUPDATE history_items SET html = content WHERE kind = 5 AND html = '';\nPRAGMA user_version = 2;\nCOMMIT;",
			created:   daysAgoAt(23, 11, 44),
			syncState: history.SyncStateOffline,
		},
		{
			id:        "preview-link-dribbble",
			kind:      history.KindLink,
			text:      "https://dribbble.com/shots/24118837-clipboard-manager-dashboard",
			created:   daysAgoAt(22, 19, 2),
			favorite:  true,
			syncState: history.SyncStateSynced,
		},
		{
			id:       "preview-file-sketch",
			kind:     history.KindFile,
			text:     "~/Documents/Design/GoPaste 图标.sketch",
			fileSize: 28_311_552,
			source:   "preview-imac-design",
			created:  daysAgoAt(19, 13, 27),
		},
		{
			id:       "preview-file-keynote",
			kind:     history.KindFile,
			text:     "~/Desktop/路线图.key",
			fileSize: 61_931_520,
			created:  daysAgoAt(17, 16, 51),
		},
		{
			id:        "preview-note-onboarding-en",
			kind:      history.KindText,
			text:      "Open GoPaste on two machines in the same network, pair them once, and clipboard history starts syncing.\nNo account, no relay, no cloud.",
			created:   daysAgoAt(15, 8, 19),
			syncState: history.SyncStateSynced,
		},
		{
			id:        "preview-rich-changelog",
			kind:      history.KindRichText,
			text:      "更新日志\n0.1.0\n- 局域网直连同步\n- 剪贴板历史筛选栏\n- 设备页同步方向设置\n0.0.9\n- 首个可运行版本",
			html:      "<h3>更新日志</h3><h4>0.1.0</h4><ul><li>局域网直连同步<ul><li>剪贴板历史筛选栏</li><li>设备页同步方向设置</li></ul></li></ul><h4>0.0.9</h4><ul><li>首个可运行版本</li></ul>",
			created:   daysAgoAt(13, 20, 36),
			syncState: history.SyncStatePartial,
		},
		{
			id:      "preview-image-earth-month",
			kind:    history.KindImage,
			image:   "image-earth-iss.png",
			source:  "preview-ubuntu-laptop",
			created: daysAgoAt(9, 18, 12),
		},
		{
			id:        "preview-file-svelte-component",
			kind:      history.KindFile,
			text:      "~/Projects/web/DeviceCard.svelte",
			fileSize:  4_664,
			created:   daysAgoAt(10, 10, 33),
			syncState: history.SyncStateSynced,
		},
		{
			id:       "preview-file-rust-daemon",
			kind:     history.KindFile,
			text:     "~/Projects/daemon/src/main.rs",
			fileSize: 9_027,
			source:   "preview-fedora",
			created:  daysAgoAt(11, 15, 46),
		},
		{
			id:       "preview-file-php-site",
			kind:     history.KindFile,
			text:     "~/Projects/site/index.php",
			fileSize: 6_318,
			created:  daysAgoAt(13, 9, 54),
		},
		{
			id:        "preview-file-sql-migration-file",
			kind:      history.KindFile,
			text:      "~/Documents/GoPaste/migrations/002-html.sql",
			fileSize:  486,
			created:   daysAgoAt(14, 11, 21),
			syncState: history.SyncStateOffline,
		},
		{
			id:       "preview-file-html-page",
			kind:     history.KindFile,
			text:     "~/Projects/site/index.html",
			fileSize: 3_942,
			source:   "preview-ubuntu-workstation",
			created:  daysAgoAt(16, 17, 9),
		},
		{
			id:       "preview-file-css-theme",
			kind:     history.KindFile,
			text:     "~/Projects/site/app.css",
			fileSize: 11_264,
			created:  daysAgoAt(18, 14, 37),
		},
		{
			id:        "preview-file-sass-theme",
			kind:      history.KindFile,
			text:      "~/Projects/site/theme.scss",
			fileSize:  2_105,
			created:   daysAgoAt(20, 16, 2),
			syncState: history.SyncStatePartial,
		},
		{
			id:       "preview-file-plist-xml",
			kind:     history.KindFile,
			text:     "~/Documents/GoPaste/com.example.gopaste.plist",
			fileSize: 1_884,
			source:   "preview-mac",
			created:  daysAgoAt(24, 13, 15),
		},
		{
			id:       "preview-file-blender-model",
			kind:     history.KindFile,
			text:     "~/Documents/3D/图标建模.blend",
			fileSize: 9_437_184,
			source:   "preview-imac-design",
			created:  daysAgoAt(28, 19, 43),
		},
		// —— 更早 ——
		{
			id:      "preview-note-legacy-migration",
			kind:    history.KindText,
			text:    "迁移记录：旧的 JSON 历史文件已导入 SQLite，图片以 PNG 形式内联保存。",
			created: daysAgoAt(40, 11, 20),
		},
		{
			id:      "preview-code-java-record",
			kind:    history.KindCode,
			text:    "record Device(String id, String name, boolean online) {}\n\nList<Device> online = devices.stream()\n    .filter(Device::online)\n    .sorted(Comparator.comparing(Device::name))\n    .toList();",
			source:  "preview-thinkpad",
			created: daysAgoAt(55, 11, 9),
		},
		{
			id:        "preview-file-archive-zip",
			kind:      history.KindFile,
			text:      "~/Documents/archive-2024.zip",
			fileSize:  5_242_880,
			created:   daysAgoAt(90, 18, 30),
			favorite:  true,
			syncState: history.SyncStateReplaced,
		},
		{
			id:      "preview-link-legacy-import",
			kind:    history.KindLink,
			text:    "https://docs.example.com/clipboard-history/legacy-import",
			source:  "preview-ubuntu-laptop",
			created: daysAgoAt(120, 10, 15),
		},
		{
			id:      "preview-rich-quarterly-report",
			kind:    history.KindRichText,
			text:    "季度回顾\n- 完成三平台打包\n- 完成设备配对流程\n- 剪贴板历史改为本地 SQLite",
			html:    "<h2>季度回顾</h2>\n<ul><li>完成三平台打包</li><li>完成设备配对流程</li><li>剪贴板历史改为本地 SQLite</li></ul>",
			created: daysAgoAt(210, 14, 45),
		},
		{
			id:      "preview-code-perl",
			kind:    history.KindCode,
			text:    "my @online = grep { $_->{online} } @devices;\nprint scalar(@online), \" devices online\\n\";",
			source:  "preview-fedora",
			created: daysAgoAt(75, 10, 22),
		},
		{
			id:      "preview-code-vimscript",
			kind:    history.KindCode,
			text:    "nnoremap <silent> <leader>gp :<C-u>call GoPaste#History()<CR>\nlet g:gopaste_keep_register = v:true",
			created: daysAgoAt(110, 9, 47),
		},
		{
			id:      "preview-link-old-blog",
			kind:    history.KindLink,
			text:    "https://blog.example.com/2024/09/why-clipboard-managers-should-stay-local",
			created: daysAgoAt(150, 20, 33),
		},
		{
			id:       "preview-file-old-dmg",
			kind:     history.KindFile,
			text:     "~/Downloads/GoPaste-0.0.9.dmg",
			fileSize: 24_117_248,
			source:   "preview-mac",
			created:  daysAgoAt(180, 15, 58),
			favorite: true,
		},
		{
			id:      "preview-note-old-todo",
			kind:    history.KindText,
			text:    "早期待办\n[ ] 图标改成矢量\n[x] 窗口尺寸可调\n[x] 历史记录支持筛选",
			created: daysAgoAt(240, 12, 6),
		},
		{
			id:      "preview-image-old-nebula",
			kind:    history.KindImage,
			image:   "image-nebula-trifid.png",
			created: daysAgoAt(300, 21, 25),
		},
		{
			id:       "preview-file-swift-app",
			kind:     history.KindFile,
			text:     "~/Projects/GoPaste/Device.swift",
			fileSize: 7_733,
			source:   "preview-thinkpad",
			created:  daysAgoAt(65, 11, 30),
		},
		{
			id:       "preview-file-mygo-toml",
			kind:     history.KindFile,
			text:     "~/Projects/GoPaste/mygo.toml",
			fileSize: 742,
			created:  daysAgoAt(130, 16, 11),
		},
		{
			id:       "preview-file-unknown-ext",
			kind:     history.KindFile,
			text:     "~/Downloads/thing.qqq",
			fileSize: 20_480,
			created:  daysAgoAt(200, 10, 5),
		},
		{
			id:       "preview-file-no-ext",
			kind:     history.KindFile,
			text:     "~/Downloads/扫描件 001",
			fileSize: 8_388_608,
			source:   "preview-windows-finance",
			created:  daysAgoAt(260, 9, 18),
		},
	}
}

// folderCopyPaths 返回一次「复制整个文件夹」会得到的路径列表。
//
// 行数刻意超过预览区一次列出的上限，用来走查末尾的「还有 N 个」汇总行：
// 选中一个文件夹时，剪贴板里是几百条路径，预览不能每条都建一行。
func folderCopyPaths() string {
	names := []string{
		"AGENTS.md",
		"code_view.go",
		"device_view.go",
		"go.mod",
		"go.sum",
		"i18n.go",
		"icons.go",
		"main.go",
		"model.go",
		"mygo.json",
		"persistence.go",
		"preview_data.go",
		"runtime.go",
		"settings_view.go",
		"state.go",
		"theme.go",
		"tray.go",
		"ui_tokens.go",
		"view.go",
		"zoom.go",
		"internal/capture/capture.go",
		"internal/capture/digest.go",
		"internal/capture/read.go",
		"internal/clipboard/clipboard.go",
		"internal/history/repository.go",
		"internal/history/store.go",
		"internal/history/sync.go",
		"internal/history/sqlite/sqlite.go",
	}
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, "~/Projects/GoPaste/"+name)
	}
	return strings.Join(paths, "\n")
}

// validateFixture 检查手写的预览记录本身是否自洽，避免错误悄悄写进数据库。
func validateFixture(entries []fixture) error {
	seen := make(map[string]bool, len(entries))
	kinds := make(map[history.Kind]bool, len(kindOrder))
	for _, entry := range entries {
		if !strings.HasPrefix(entry.id, previewIDPrefix) {
			return fmt.Errorf("记录 ID 必须以 %s 开头: %s", previewIDPrefix, entry.id)
		}
		if seen[entry.id] {
			return fmt.Errorf("记录 ID 重复: %s", entry.id)
		}
		seen[entry.id] = true

		if _, ok := kindLabels[entry.kind]; !ok {
			return fmt.Errorf("%s 的类型取值无效: %d", entry.id, entry.kind)
		}
		kinds[entry.kind] = true

		if entry.source != "" && !isPreviewDevice(entry.source) {
			return fmt.Errorf("%s 的来源设备不在预览设备列表里: %s", entry.id, entry.source)
		}
		if entry.source != "" && entry.syncState != history.SyncStateNone {
			// 只有本机捕获的记录才会投递给配对设备；收到的记录已经到达本机。
			return fmt.Errorf("%s 来自其他设备，不应该带同步状态", entry.id)
		}
		if !validSyncState(entry.syncState) {
			return fmt.Errorf("%s 的同步状态取值无效: %d", entry.id, entry.syncState)
		}

		if entry.kind == history.KindImage && entry.image == "" {
			// 图片条目必须来自真实 PNG：不写没有图片的占位记录。
			return fmt.Errorf("%s 是图片条目但没有指定图片文件", entry.id)
		}
		if entry.kind != history.KindImage && entry.image != "" {
			return fmt.Errorf("%s 不是图片条目，却指定了图片文件", entry.id)
		}
		if entry.kind != history.KindRichText && entry.html != "" {
			return fmt.Errorf("%s 不是富文本条目，却带了 html", entry.id)
		}
		if entry.kind == history.KindRichText && (entry.text == "" || entry.html == "") {
			return fmt.Errorf("%s 是富文本条目，必须同时有纯文本和 html", entry.id)
		}
	}

	missing := make([]string, 0, len(kindOrder))
	for _, kind := range kindOrder {
		if !kinds[kind] {
			missing = append(missing, kindLabels[kind])
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("预览数据缺少内容类型: %s", strings.Join(missing, "、"))
	}
	return nil
}

// resolveCreated 把相对时间描述换算成本地时间。
//
// 时间戳一律落在整分钟上：今天的记录取「今天已经过去的时间」的固定比例，所以
// 任何时刻运行都落在今天、且在当下之前；整分钟让同一分钟内的连续两次运行写出
// 完全一样的行。
func resolveCreated(spec createdSpec, now time.Time) time.Time {
	if spec.today {
		startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		elapsed := now.Sub(startOfToday)
		moment := startOfToday.Add(time.Duration(float64(elapsed) * spec.fraction))
		return moment.Truncate(time.Minute)
	}
	day := now.AddDate(0, 0, -spec.daysAgo)
	return time.Date(day.Year(), day.Month(), day.Day(), spec.hour, spec.minute, 0, 0, now.Location())
}

// deliveriesFor 按同步状态生成投递记录，两者的取值必须一致。本机捕获的记录才
// 会投递给配对设备；来自其他设备的记录已经到达本机，没有投递记录。
func deliveriesFor(entry fixture) []history.DeviceDelivery {
	if entry.source != "" {
		return nil
	}
	deliveries := make([]history.DeviceDelivery, 0, len(previewDeviceIDs))
	switch entry.syncState {
	case history.SyncStateSynced:
		// 每台目标设备都已接收。
		for _, deviceID := range previewDeviceIDs {
			deliveries = append(deliveries, history.DeviceDelivery{
				DeviceID: deviceID,
				State:    history.DeliveryReceived,
			})
		}
	case history.SyncStatePartial:
		// 在线设备已接收，离线设备还在等待。
		for _, deviceID := range previewDeviceIDs {
			state := history.DeliveryOffline
			if onlineDeviceIDs[deviceID] {
				state = history.DeliveryReceived
			}
			deliveries = append(deliveries, history.DeviceDelivery{
				DeviceID: deviceID,
				State:    state,
			})
		}
	case history.SyncStateOffline:
		// 没有设备收到：在线设备待发送，离线设备标记为离线。
		for _, deviceID := range previewDeviceIDs {
			state := history.DeliveryPending
			if !onlineDeviceIDs[deviceID] {
				state = history.DeliveryOffline
			}
			deliveries = append(deliveries, history.DeviceDelivery{
				DeviceID: deviceID,
				State:    state,
			})
		}
	default:
		// SyncStateNone 和 SyncStateReplaced 都没有投递记录：前者没有同步，
		// 后者从未送达任何设备。
		return nil
	}
	return deliveries
}

// buildItems 把预览记录展开成 history.Item，并按需读入真实 PNG。
func buildItems(entries []fixture, imagesDir string, now time.Time) ([]history.Item, error) {
	items := make([]history.Item, 0, len(entries))
	for _, entry := range entries {
		item := history.Item{
			ID:             entry.id,
			Kind:           entry.kind,
			Text:           entry.text,
			HTML:           entry.html,
			FileSize:       entry.fileSize,
			SourceDeviceID: entry.source,
			CreatedAt:      resolveCreated(entry.created, now),
			Favorite:       entry.favorite,
			Sync:           entry.syncState,
			Deliveries:     deliveriesFor(entry),
		}
		if entry.image != "" {
			png, width, height, err := readPNG(filepath.Join(imagesDir, entry.image))
			if err != nil {
				return nil, err
			}
			item.Image = &history.Image{PNG: png, Width: width, Height: height}
		}
		items = append(items, item)
	}
	return items, nil
}

// readPNG 读取一张真实 PNG，并用 image.DecodeConfig 读出它的宽高；绝不伪造
// 图片字节。
func readPNG(path string) ([]byte, int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("无法读取预览图片 %s: %w", path, err)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("无法解析预览图片 %s: %w", path, err)
	}
	if format != "png" {
		return nil, 0, 0, fmt.Errorf("预览图片 %s 不是 PNG 文件（实际是 %s）", path, format)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return nil, 0, 0, fmt.Errorf("预览图片 %s 的尺寸无效: %dx%d", path, config.Width, config.Height)
	}
	return data, config.Width, config.Height, nil
}

// requireImagesDir 确认图片目录存在且至少含有一张 PNG。
func requireImagesDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("图片目录不存在：%s\n预览图片必须来自真实 PNG 文件（不会写入没有图片的占位记录）；把预览用的 PNG 放进这个目录，或用 -images DIR 指向别处。", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("无法读取图片目录 %s: %w", dir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
			return nil
		}
	}
	return fmt.Errorf("图片目录里没有任何 PNG 文件：%s\n预览图片必须来自真实 PNG 文件，不会写入没有图片的占位记录。", dir)
}

func isPreviewDevice(deviceID string) bool {
	for _, id := range previewDeviceIDs {
		if id == deviceID {
			return true
		}
	}
	return false
}

func validSyncState(state history.SyncState) bool {
	switch state {
	case history.SyncStateNone, history.SyncStateSynced, history.SyncStatePartial,
		history.SyncStateOffline, history.SyncStateReplaced:
		return true
	default:
		return false
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "错误："+err.Error())
		os.Exit(1)
	}
}

func run() error {
	out := flag.String("out", defaultOutputPath,
		"输出数据库文件；每次运行都会先删除同名文件，重新生成一份全新的数据库")
	images := flag.String("images", defaultImagesDir,
		"预览图片目录；必须存在且至少包含一张 PNG")
	flag.Parse()

	imagesDir, err := filepath.Abs(*images)
	if err != nil {
		return fmt.Errorf("无法解析图片目录 %s: %w", *images, err)
	}
	if err := requireImagesDir(imagesDir); err != nil {
		return err
	}

	entries := buildFixture()
	if err := validateFixture(entries); err != nil {
		return err
	}

	// 先读图片再动数据库：缺图时不会留下半份数据。
	now := time.Now()
	items, err := buildItems(entries, imagesDir, now)
	if err != nil {
		return err
	}

	outPath, err := filepath.Abs(*out)
	if err != nil {
		return fmt.Errorf("无法解析输出路径 %s: %w", *out, err)
	}
	// 输出目录可能还不存在，按应用自己的做法建出来：目录只给当前用户。
	if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
		return fmt.Errorf("无法创建输出目录 %s: %w", filepath.Dir(outPath), err)
	}
	// 每次运行都删掉旧文件，重新生成一份全新的数据库。WAL 的附属文件一起删：
	// 上一份数据可能是在应用崩溃时留下的，只剩 WAL 的文件不该留在新库旁边。
	for _, stale := range []string{outPath, outPath + "-wal", outPath + "-shm"} {
		if err := os.Remove(stale); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("无法删除旧的文件 %s: %w", stale, err)
		}
	}

	ctx := context.Background()
	repository, err := sqlite.Open(ctx, outPath)
	if err != nil {
		return fmt.Errorf("无法打开数据库 %s: %w", outPath, err)
	}
	if err := repository.Put(ctx, items...); err != nil {
		_ = repository.Close()
		return fmt.Errorf("无法写入数据库 %s: %w", outPath, err)
	}
	if err := repository.Close(); err != nil {
		return fmt.Errorf("无法关闭数据库 %s: %w", outPath, err)
	}

	printSummary(outPath, items)
	return nil
}

// printSummary 打印输出路径、各类型的行数和启动应用的提示。
func printSummary(outPath string, items []history.Item) {
	counts := make(map[history.Kind]int, len(kindOrder))
	deliveries := 0
	for _, item := range items {
		counts[item.Kind]++
		deliveries += len(item.Deliveries)
	}

	fmt.Printf("数据库：%s\n", outPath)
	fmt.Printf("已写入 %d 条记录、%d 条投递记录；每次运行都会先删除旧文件，生成一份全新的数据库。\n",
		len(items), deliveries)
	fmt.Println("按类型统计：")
	for _, kind := range kindOrder {
		fmt.Printf("  %d %s: %d\n", kind, kindLabels[kind], counts[kind])
	}
	fmt.Printf("提示：scripts/preview.sh 会重建这份数据并启动开发版应用。\n")
	fmt.Printf("      只用这份数据库启动：GOPASTE_HISTORY_DB=%s GOFLAGS='-tags=preview' go run .\n", outPath)
}
