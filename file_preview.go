package main

import (
	"gopaste/internal/filepreview"
	"gopaste/internal/history"

	"github.com/egoist/mygo/ui"
)

// 文件预览按需生成，像 Finder 的快速查看：选中一条文件记录，预览区就显示文件本身
// 的内容。文件内容不在记录里，所以要读盘与解码——这些都在界面线程之外做，结果回到
// 界面线程后缓存在内存里，同一次运行里再看同一份文件不再重算。

// filePreviewEntry 是一条文件记录本次运行里算出来的预览结果：图片已经解码成位图
// （免得每帧重新解码 PNG），文本就是文件内容。
type filePreviewEntry struct {
	bitmap *ui.Bitmap
	text   string
	// failed 表示这条记录没有可显示的内容：格式本来不支持、文件已删、内容是二进制。
	// 记住它，免得每帧都重新判断、重新起后台任务。
	failed bool
}

// filePreviewFor 返回一条文件记录该显示的预览，还没有就算一次。
//
// 预览区每帧问一次：算过的直接返回；没算过的请求一次后台生成，并在返回空结果时
// 让预览区显示「正在生成预览…」（见 filePreviewGenerating）。
func (a *application) filePreviewFor(item history.Item) filePreviewEntry {
	if entry, ok := a.filePreviews[item.ID]; ok {
		return entry
	}
	a.requestFilePreview(item)
	return a.filePreviews[item.ID]
}

// requestFilePreview 请运行时在界面线程之外生成一条记录的预览。
//
// 读盘与解码都不能发生在画帧的地方，所以这里只登记一次请求：同一条记录在生成期间
// 不重复请求，已知没有内容的记录也不再请求。格式决定不了一律不请求——压缩包、
// 字体这类文件本来就没有可显示的内容，不该为它们起 goroutine。
func (a *application) requestFilePreview(item history.Item) {
	if item.Kind != kindFile || item.ID == "" || a.onFilePreview == nil {
		return
	}
	if _, pending := a.filePreviewPending[item.ID]; pending {
		return
	}
	path := firstFilePath(item.Text)
	if path == "" || !filepreview.CanPreview(path) {
		a.rememberFilePreview(item.ID, filePreviewEntry{failed: true})
		return
	}
	if a.filePreviewPending == nil {
		a.filePreviewPending = make(map[string]struct{})
	}
	a.filePreviewPending[item.ID] = struct{}{}
	a.onFilePreview(item.ID, path)
}

// applyFilePreview 把一次后台生成的结果放进缓存。运行时经 Window.Update 回到
// 界面线程后调用它。
//
// 图片在这里解码成位图：字节已经在内存里，解码是纯 CPU 工作，和图片条目走同一条路
// （见 decodeBitmap）。生成不出东西就记成没有内容，预览区退回「无法预览此文件」。
func (a *application) applyFilePreview(id string, preview *filepreview.Preview) {
	delete(a.filePreviewPending, id)
	entry := filePreviewEntry{}
	switch {
	case preview == nil:
		entry.failed = true
	case len(preview.PNG) > 0:
		bitmap, err := ui.DecodeBitmap(preview.PNG)
		if err != nil {
			entry.failed = true
		} else {
			entry.bitmap = bitmap
		}
	case preview.Text != "":
		entry.text = preview.Text
	default:
		entry.failed = true
	}
	a.rememberFilePreview(id, entry)
}

// rememberFilePreview 存下一条记录的预览结果。记录可能在生成期间被删掉，缓存里
// 多出来的那一项由删除路径清掉（见 forgetFilePreview）。
func (a *application) rememberFilePreview(id string, entry filePreviewEntry) {
	if a.filePreviews == nil {
		a.filePreviews = make(map[string]filePreviewEntry)
	}
	a.filePreviews[id] = entry
}

// forgetFilePreview 丢掉一条记录的预览缓存，记录被删掉时调用。
func (a *application) forgetFilePreview(id string) {
	delete(a.filePreviews, id)
	delete(a.filePreviewPending, id)
}

// filePreviewGenerating 报告一条记录的预览是否正在生成，预览区据此把
// 「无法预览此文件」换成「正在生成预览…」。
func (a *application) filePreviewGenerating(id string) bool {
	_, pending := a.filePreviewPending[id]
	return pending
}
