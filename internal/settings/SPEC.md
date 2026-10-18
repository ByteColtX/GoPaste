---
id: settings-file
type: submodule-design
parent: gopaste
title: 设置文件（internal/settings）
status: draft
tags:
  - settings
  - persistence
---

# 设置文件

## 职责

把一个很小的 JSON 文件读成结构体、再写回去：`settings.json`，与 `history.db` 同目录
（`<用户数据目录>/GoPaste/`）。它只存跨启动必须记住的几项，不碰剪贴板内容。

## 文件契约

```json
{
  "onboarding": { "completed": true },
  "deviceName": "bytecolt-mbp",
  "retention": { "maxItems": 500 }
}
```

- 缺字段即默认值，所以旧版本写的文件仍能读。
- `deviceName` 为空时不写进文件（`omitempty`）：空字段和「没设过」是同一件事，写出来
  只会让文件多一行噪音。
- `retention` 存历史记录的自动清理上限（见 `gopaste`）。两个维度各自可缺：缺
  `maxItems` 或 `maxDays` 表示用户没选过这一维，应用继续用自己的默认值；写了 `0`
  表示明确的「不限」。字段用指针而不是 `int`，就是为了让这两种情况能分开——用
  `int` 时旧文件会读成「不限」，等于把所有老用户的保留策略静默关掉。
- 只有用户真改过的那一维才落盘，默认值不写进文件：将来调默认值，没选过的人跟得上。
- 配对码不在这里：配对由 `internal/lansync` 的设备页流程完成，配对结果落在
  `devices.json`（身份与钉住的公钥）。

## 行为契约

- `Load`：文件不存在**不是错误**，返回零值——首次运行就是「没有设置」。文件存在但
  读不出来或解析不了**是**错误，由调用方决定后果（应用按首次启动处理）。
- `Save`：先写同目录临时文件再 `rename`，崩溃不会留下半截文件被下次启动误读成首次
  运行；目录 0700、文件 0600。

## 边界

- 不依赖 MyGo、不联网，可以脱离界面单独测试。
- 不负责判断「该不该显示向导」：它只报告文件里的事实，由应用外壳决定（见 `gopaste`）。
- 不存界面偏好。语言、外观、删除前确认目前仍只在本次运行内有效；`retention` 是
  例外：它决定的是数据留多久，不是界面长什么样，所以必须跨启动记住。
