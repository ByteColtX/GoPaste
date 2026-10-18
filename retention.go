package main

import (
	"gopaste/internal/history"
	"gopaste/internal/settings"
)

// 默认保留策略：最多一万条、最多九十天。两个维度同时生效，谁先到就删谁；收藏
// 的记录既不删也不占条数额度。
const (
	defaultRetentionItems = 10000
	defaultRetentionDays  = 90
)

// defaultRetention 返回用户没动过保留策略时的自动清理上限。
func defaultRetention() history.RetentionPolicy {
	return history.RetentionPolicy{MaxItems: defaultRetentionItems, MaxDays: defaultRetentionDays}
}

// pruneHistory 按当前保留策略裁掉超额记录，并把删除排进写队列。
//
// 它只在界面线程上调用：规则和落盘各归一处——Store 决定删哪些，唯一的写入
// goroutine 负责真的从数据库里删掉。被删记录的位图与高亮缓存一并丢掉，免得已经
// 不存在的记录还占着内存里的图片。
func (a *application) pruneHistory() {
	if a.history == nil {
		return
	}
	removed := a.history.ApplyRetention(a.retention)
	if len(removed) == 0 {
		return
	}
	for _, id := range removed {
		delete(a.historyBitmaps, id)
		delete(a.codeHighlights, id)
	}
	a.persistDelete(removed...)
}

// retentionSettings 返回要写进设置文件的那一段保留策略，只写用户真改过的维度：
// 没改过的维度留空，默认值继续跟着程序走，而不是在第一次保存时被冻在文件里。
func (a *application) retentionSettings() *settings.Retention {
	defaults := defaultRetention()
	retention := settings.Retention{}
	if a.retention.MaxItems != defaults.MaxItems {
		retention.MaxItems = retentionValue(a.retention.MaxItems)
	}
	if a.retention.MaxDays != defaults.MaxDays {
		retention.MaxDays = retentionValue(a.retention.MaxDays)
	}
	if retention.MaxItems == nil && retention.MaxDays == nil {
		return nil
	}
	return &retention
}

// applyRetentionSettings 用设置文件里的保留策略覆盖默认值。没写的维度保持默认：
// 旧版本写的文件、以及只改过一项的文件都是这种情况。
func (a *application) applyRetentionSettings(retention *settings.Retention) {
	if retention == nil {
		return
	}
	if retention.MaxItems != nil {
		a.retention.MaxItems = *retention.MaxItems
	}
	if retention.MaxDays != nil {
		a.retention.MaxDays = *retention.MaxDays
	}
}

// retentionValue 返回 v 的地址，用来在设置文件里区分「没设过」与「设成 0」。
func retentionValue(v int) *int { return &v }

// applyRetention 在用户改完保留上限后立刻生效：先裁一次，再把选择落盘。
//
// 调低上限的那一刻就可能删掉一大批旧记录，这是保留策略的本意；设置行与下面的
// 说明都写了“立即删除且不可撤销”，所以这里不再弹二次确认。
func (a *application) applyRetention() {
	a.pruneHistory()
	a.saveSettings()
}
