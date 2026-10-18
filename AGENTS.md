# AGENTS.md

## 项目

GoPaste 是使用 Go 和 MyGo 原生 UI 编写的 macOS、Windows、Linux 剪贴板同步软件。

## 固定约束

- 同步只允许已配对设备之间的局域网直连。
- 永不加入内置中继、云端服务、第三方代理或非局域网回退连接。
- 保持 MyGo 原生窗口和控件，不引入 WebView 或独立前端框架。
- 剪贴板内容属于敏感数据，不写入普通日志、遥测或第三方服务。
- 平台专属代码应隔离，不能破坏其他平台的构建和行为。

## 常用命令

```sh
go fmt ./...
go tool mygo vet .
CGO_ENABLED=0 go test ./...
go run .
```

## 本地预览数据

界面走查用的剪贴板历史由 `scripts/mockdb` 生成到 `.preview/history.db`（已被
.gitignore 忽略，生成器每次重建目标文件）。它故意不放在 `resources/` 下：
mygo dev 递归监视那个目录，应用运行时的每次写入都会让它重建并重启，
`resources/` 里的东西也会被打进应用包。

```sh
scripts/preview.sh      # 生成预览数据并启动开发版应用
```

它等价于下面两步。`GOPASTE_HISTORY_DB`（或 `go run . -db PATH`）让应用读写
指定的数据库文件，不指定时仍是用户数据目录下的 `history.db`：路径必须是绝对
路径，应用的工作目录不由调用方决定。应用启动的第一行日志会报出它实际用的
文件，界面里没有数据时先看那一行。

```sh
go run ./scripts/mockdb
GOPASTE_HISTORY_DB="$PWD/.preview/history.db" go tool mygo dev
```

预览数据只造历史记录，不造设备：设备列表来自真实的局域网发现，没配对过就是空的。

## 开发规则

- 网络代码只放在 `internal/lansync`：身份、mDNS 发现、配对、TLS 连接、投递都在那里，
  `internal/` 下其余包不碰网络。界面通过 `lansync.Service` 使用它，回调一律经
  `Window.Update` 回到主线程。
- 依赖 MyGo API 时以当前 `go.mod` 版本和模块源码为准。
- MyGo 元素只在当前构建帧有效；持久状态中保存数据、`ui.Handle` 或 `ui.ListState`，不要保存元素。
- 控件使用稳定且唯一的 key；渲染函数不要无条件执行文件、剪贴板或网络副作用。
- 慢任务放在 UI 线程之外，结果通过 `Window.Update` 发布，并处理窗口关闭和过期结果。
- 遵循 `gofmt` 和标准 Go 风格；避免无关重构和依赖变更。

## 提交前

- 检查 `git diff`，不要提交构建产物、`.mygo` 状态、Node 依赖或本地 demo 数据。
- 确认改动没有引入云端、中继、遥测或非局域网连接。
