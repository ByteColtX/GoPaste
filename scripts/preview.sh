#!/bin/sh
# 生成预览数据，然后用 mygo dev 打开它。
#
# 这两步手动做是这样的：
#
#   go run ./scripts/mockdb
#   GOPASTE_HISTORY_DB=.preview/history.db GOFLAGS='-tags=preview' go tool mygo dev
#
# 用脚本是为了路径不会写错：变量必须是绝对路径，应用的工作目录不由调用方决定。
# 多出来的参数传给 scripts/mockdb，例如 scripts/preview.sh -images /tmp/imgs。
set -e

root=$(cd "$(dirname "$0")/.." && pwd)
db="$root/.preview/history.db"

cd "$root"
go run ./scripts/mockdb -out "$db" "$@"

echo
echo "启动开发版应用，历史数据库：$db"
echo "（先退出已经在跑的 GoPaste Dev：单实例锁会让新实例直接退出。）"

GOPASTE_HISTORY_DB="$db" GOFLAGS="${GOFLAGS:+$GOFLAGS }-tags=preview" go tool mygo dev
