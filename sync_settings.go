package main

import (
	"strconv"
	"strings"

	"gopaste/internal/settings"
)

// 同步页的文件缓存设置：配额、保留期限、自动清理与保存位置。这里只负责把它们
// 落进内存状态并落盘，真正的清理在 files_cache.go 里做。

// applyFileQuota 把配额（MB）变成输入框里的文本与生效的字节数。小于最小值表示
// 设置文件里没写，用默认值。
func (a *application) applyFileQuota(megabytes int) {
	if megabytes < settings.MinFileQuotaMB {
		megabytes = settings.DefaultFileQuotaMB
	}
	a.fileQuotaMB = strconv.Itoa(megabytes)
	a.fileQuotaBytes = settings.FileQuotaBytes(megabytes)
}

// editFileQuota 处理配额输入的变化：只接受不小于最小值的整数，不合法时保留上一个
// 有效值（输入框里的字不动，用户还能接着改）。
func (a *application) editFileQuota(text string) {
	megabytes, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || megabytes < settings.MinFileQuotaMB {
		return
	}
	a.fileQuotaBytes = settings.FileQuotaBytes(megabytes)
	a.saveSettings()
}

// editFileRetention 处理保留期限下拉的变化：0 是「不限」，其余是小时数。改完立刻
// 按新期限裁一次，与历史保留策略改完就裁是同一个约定。
func (a *application) editFileRetention(hours int) {
	a.fileRetentionHours = max(hours, 0)
	a.saveSettings()
}

// editFileAutoClean 开关「自动清理」。
func (a *application) editFileAutoClean(enabled bool) {
	a.fileAutoClean = enabled
	a.saveSettings()
}

// setDefaultFilesDir 记下默认目录，并据此解析当前真正落盘的根目录。运行时在启动
// 同步服务之前调用它：设置文件里选过的目录要到这一刻才有默认位置可以对照。
func (a *application) setDefaultFilesDir(dir string) {
	a.fileSyncDirDefault = dir
	a.applyFileSyncDir(a.fileSyncDir)
}

// applyFileSyncDir 让一个保存位置生效：空串表示用默认目录。换目录只影响之后收到的
// 文件，已经落盘的记录继续指向它们自己的路径。
func (a *application) applyFileSyncDir(dir string) {
	a.fileSyncDir = dir
	root := dir
	if root == "" {
		root = a.fileSyncDirDefault
	}
	a.receivedFilesDir = root
	if a.syncService != nil {
		a.syncService.SetFilesDir(root)
	}
}

// setFileSyncDir 是用户选完保存位置后的入口：立即生效并落盘。
func (a *application) setFileSyncDir(dir string) {
	a.applyFileSyncDir(dir)
	a.saveSettings()
}

// resetFileSyncDir 把保存位置恢复成默认目录。
func (a *application) resetFileSyncDir() {
	a.setFileSyncDir("")
}

// applySyncSettings 用设置文件里的同步设置覆盖内存状态。缺省值保持默认：旧版本
// 写的文件、以及只改过其中一项的文件都是这种情况。
func (a *application) applySyncSettings(sync *settings.Sync) {
	a.syncEnabled = true
	a.autoSendEnabled = true
	a.fileSyncEnabled = true
	a.fileAutoClean = true
	a.fileRetentionHours = settings.DefaultFileRetentionHours
	if sync == nil {
		a.applyFileSyncDir("")
		return
	}
	if sync.Enabled != nil {
		a.syncEnabled = *sync.Enabled
	}
	if sync.AutoSend != nil {
		a.autoSendEnabled = *sync.AutoSend
	}
	if sync.Files == nil {
		a.applyFileSyncDir("")
		return
	}
	if sync.Files.Enabled != nil {
		a.fileSyncEnabled = *sync.Files.Enabled
	}
	if sync.Files.AutoClean != nil {
		a.fileAutoClean = *sync.Files.AutoClean
	}
	if sync.Files.RetentionHours != nil {
		a.fileRetentionHours = max(*sync.Files.RetentionHours, 0)
	}
	a.applyFileQuota(sync.Files.QuotaMB)
	a.applyFileSyncDir(sync.Files.Dir)
}

// syncSettingsFile 把同步页的选择投影成设置文件里的 sync 对象。全是默认值时返回
// nil：那样文件里就不会多出一个空对象，将来调默认值，没改过的人跟得上。
func (a *application) syncSettingsFile() *settings.Sync {
	sync := settings.Sync{}
	if !a.syncEnabled {
		off := false
		sync.Enabled = &off
	}
	if !a.autoSendEnabled {
		off := false
		sync.AutoSend = &off
	}
	files := settings.SyncFiles{}
	filesChanged := false
	if !a.fileSyncEnabled {
		off := false
		files.Enabled = &off
		filesChanged = true
	}
	if a.fileSyncDir != "" {
		files.Dir = a.fileSyncDir
		filesChanged = true
	}
	if megabytes := a.fileQuotaBytes >> 20; megabytes > 0 && megabytes != settings.DefaultFileQuotaMB {
		files.QuotaMB = int(megabytes)
		filesChanged = true
	}
	if a.fileRetentionHours != settings.DefaultFileRetentionHours {
		hours := a.fileRetentionHours
		files.RetentionHours = &hours
		filesChanged = true
	}
	if !a.fileAutoClean {
		off := false
		files.AutoClean = &off
		filesChanged = true
	}
	if filesChanged {
		sync.Files = &files
	}
	if sync.Enabled == nil && sync.AutoSend == nil && sync.Files == nil {
		return nil
	}
	return &sync
}
