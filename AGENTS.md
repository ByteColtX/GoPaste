# AGENTS.md

GoPaste 是用 Go 和 MyGo 原生 UI 写的剪贴板历史工具，目标平台 macOS、Windows、Linux。
剪贴板捕获、本地 SQLite 历史、界面已经可用；局域网同步尚未接入，设备页和配对对话框
只有界面，不要假装配对成功。

## 硬约束

这几条是产品定义，不是待办事项：

- 同步只走已配对设备之间的局域网直连；不加入中继、云端服务、第三方代理或任何非局域网回退。
- 保持 MyGo 原生窗口和控件，不引入 WebView 或独立前端框架。
- 剪贴板内容是敏感数据：不写普通日志、不做遥测、不发第三方服务。
- 平台专属代码隔离在 `tray.go`、`persistence.go`、`runtime.go`、`state.go`，不能破坏其他平台的构建和行为。

## 命令

```sh
go fmt ./...
go tool mygo vet .              # MyGo 静态检查：持久值、goroutine 捕获
CGO_ENABLED=0 go test ./...     # 必须关掉 cgo：验证纯 Go 构建
go run .                        # 运行
go tool mygo dev                # 开发模式，热重载
go tool mygo build              # 打包，macOS 产出 .app 和 .dmg
```

## 代码约定

- 依赖 MyGo API 时以 `go.mod` 里的版本和模块源码为准，不要凭记忆写。
- `internal/` 下的包不依赖 MyGo、不依赖网络，可以脱离界面单独测试；界面逻辑留在根包。
- 遵循 `gofmt` 和标准 Go 风格；不做无关重构，不擅自加依赖。

## MyGo 规则

- 元素只在当前构建帧有效。持久状态里存数据、`ui.Handle` 或 `ui.ListState`，绝不存元素。
- 控件用稳定且唯一的 key；容器的 `.Key` 要在子元素之前。
- 渲染函数不要无条件执行文件、剪贴板或网络副作用；副作用走事件处理或显式任务。
- 慢任务放 UI 线程之外，结果通过 `Window.Update` 发布，并处理窗口关闭和过期结果。

## 测试

- 改行为就改测试；`internal/` 的包用普通表驱动测试。
- UI 用 `ui.NewTester`：覆盖元素消失后重新出现、焦点、输入和快捷键。

## 提交

- 提交信息用 Conventional Commits，与现有 `git log` 一致：`feat(ui): …`、`fix(history): …`、`docs: …`
- body 只写 subject 说不出的东西（动机、被否掉的方案、副作用）；只看 subject 就能判断要不要看这个 diff 时省略。
