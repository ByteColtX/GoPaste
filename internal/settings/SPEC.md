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
  "ui": {
    "language": "en",
    "appearance": "dark",
    "confirmDelete": false,
    "zoom": 1.25
  }
}
```

- 缺字段即默认值，所以旧版本写的文件仍能读。
- `deviceName` 为空时不写进文件（`omitempty`）：空字段和「没设过」是同一件事，写出来
  只会让文件多一行噪音。
- `ui` 只承载用户改过的界面偏好，全是默认值时整个对象都不写；里面每个字段同理：
  - `language`：`"zh-Hans"` 或 `"en"`；缺省是跟随系统语言。
  - `appearance`：`"light"` 或 `"dark"`；缺省是跟随系统外观。
  - `confirmDelete`：`false` 表示删除单条记录前不再确认；缺省（或没有这个键）是确认。
  - `zoom`：界面缩放比例，`1` 是 100%；缺省是 1。
- 认不出的取值原样读回，由调用方回退到默认：坏值不让整份文件读失败，未来版本写的值
  也不会把旧版本卡住。
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
- 不存界面运行状态：窗口尺寸、侧栏展开、当前页面每次启动重新决定，不写进文件。
  `ui` 只记用户在设置页显式选过的偏好（语言、外观、删除前确认、缩放），不记这些偏好
  怎么被界面消费——那是 `gopaste` 的事。
