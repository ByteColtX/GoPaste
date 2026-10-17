package main

import (
	"flag"
	"log"
	"os"
)

// historyDatabaseEnv 指定历史数据库文件的位置。
//
// mygo dev 会把父进程的环境原样交给开发版应用，命令行参数到不了那里，
// 所以本机预览走环境变量最省事：
//
//	GOPASTE_HISTORY_DB="$PWD/.preview/history.db" go tool mygo dev
const historyDatabaseEnv = "GOPASTE_HISTORY_DB"

// parseArgs 解析本应用的启动参数，返回历史数据库文件（未指定时为空，由
// databaseFile 回落到默认位置）和要留给 mygo 的参数。
//
// 自己用掉的参数不能留在 os.Args 里：mygo 会把剩下的参数当成要打开的链接和
// 文件，一个已经存在的数据库路径会被它当成用户双击打开的文件。
func parseArgs(args []string) (string, []string) {
	flags := flag.NewFlagSet("gopaste", flag.ExitOnError)
	databasePath := flags.String("db", os.Getenv(historyDatabaseEnv),
		"历史数据库文件；默认使用用户数据目录下的 history.db")
	_ = flags.Parse(args)
	return *databasePath, flags.Args()
}

func main() {
	databasePath, rest := parseArgs(os.Args[1:])
	os.Args = append([]string{os.Args[0]}, rest...)

	if err := runApplication(databasePath); err != nil {
		log.Fatal(err)
	}
}
