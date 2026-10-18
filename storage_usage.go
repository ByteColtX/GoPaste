package main

import (
	"math"
	"os"
	"strconv"

	"gopaste/internal/filescache"
)

// storageUsage 是 GoPaste 在本机磁盘上各类数据的占用字节数。两类之和就是界面上
// 的总计：设置与配对文件只有几 KB，不进统计也不单列一行。
type storageUsage struct {
	// database 是历史数据库：history.db 与 WAL 模式带出的 -wal、-shm。剪贴板图片
	// 以 BLOB 存在库里，因此算在这一类。
	database int64
	// files 是收到的同步文件整棵目录树，每条记录一个以内容 ID 命名的子目录。
	files int64
}

// total 返回两类占用之和，也就是界面上的「总计」。
func (u storageUsage) total() int64 {
	return u.database + u.files
}

// storageUsagePaths 是测量要知道的几个位置。空字符串表示这一类本次不测（例如没有
// 数据目录、同步不可用时没有收到的文件目录），对应的占用按 0 算。
type storageUsagePaths struct {
	database string
	files    string
}

// measureStorageUsage 统计 paths 指到的文件占了多少字节。它只读元数据、只碰磁盘，
// 不认识界面，可以在界面线程之外跑。
//
// 读不到的文件按 0 算而不是报错：这是一份显示用的统计，一个刚好被删掉的记录不该让
// 整块用量变成错误态。
func measureStorageUsage(paths storageUsagePaths) storageUsage {
	var usage storageUsage
	if paths.database != "" {
		// WAL 模式的两个伴随文件与主库同生共死，加起来才是这一条记录真正占的盘。
		usage.database = fileBytes(paths.database) +
			fileBytes(paths.database+"-wal") +
			fileBytes(paths.database+"-shm")
	}
	usage.files = filescache.DirectoryBytes(paths.files)
	return usage
}

// fileBytes 返回一个文件的大小，不存在或读不到时为 0。
func fileBytes(path string) int64 {
	if path == "" {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0
	}
	return info.Size()
}

// byteUnits 是 formatBytes 依次尝试的单位，1024 进制。
var byteUnits = []string{"KB", "MB", "GB", "TB"}

// formatBytes 把字节数写成界面上的大小：1024 进制，10 以下留一位小数（`5.6 KB`），
// 10 以上取整（`177 KB`、`72 MB`），进位到 1024 时升到下一个单位。单位是固定的
// 技术缩写，不进 i18n。
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return strconv.FormatInt(bytes, 10) + " B"
	}
	value := float64(bytes) / 1024
	for index, unit := range byteUnits {
		rounded := math.Round(value*10) / 10
		last := index == len(byteUnits)-1
		if rounded >= 1024 && !last {
			value /= 1024
			continue
		}
		if rounded < 10 {
			return trimZeroDecimal(strconv.FormatFloat(rounded, 'f', 1, 64)) + " " + unit
		}
		return strconv.FormatInt(int64(math.Round(rounded)), 10) + " " + unit
	}
	// 走到这里说明 byteUnits 是空的；保留一个不会崩的兜底。
	return strconv.FormatInt(bytes, 10) + " B"
}

// trimZeroDecimal 去掉 `1.0` 末尾那位没信息的小数。
func trimZeroDecimal(text string) string {
	if len(text) > 2 && text[len(text)-2:] == ".0" {
		return text[:len(text)-2]
	}
	return text
}
