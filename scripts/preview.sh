#!/bin/sh
# 生成预览数据，然后用 mygo dev 打开它。
#
# 这两步手动做是这样的：
#
#   go run ./scripts/mockdb
#   GOPASTE_HISTORY_DB="$PWD/.preview/history.db" GOFLAGS='-tags=preview' go tool mygo dev
#
# 用脚本是为了路径不会写错：变量必须是绝对路径，应用的工作目录不由调用方决定。
# 多出来的参数传给 scripts/mockdb，例如 scripts/preview.sh -images /tmp/imgs。
set -e

root=$(cd "$(dirname "$0")/.." && pwd)
db="$root/.preview/history.db"

cd "$root"

# 提醒放在重建之前：生成器会删掉并重新写这份数据库。真在跑的实例会继续持有
# 已经被替换的文件，而单实例锁又会让新实例直接退出——先提醒，别让人对着旧窗口
# 以为数据没变。
echo "预览数据会重建到：$db"
echo "（先退出已经在跑的 GoPaste Dev，否则它看到的还是重建前的数据。）"
echo

go run ./scripts/mockdb -out "$db" "$@"

echo
echo "启动开发版应用。"

GOPASTE_HISTORY_DB="$db" GOFLAGS="${GOFLAGS:+$GOFLAGS }-tags=preview" go tool mygo dev
