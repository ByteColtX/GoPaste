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
  "pendingPairing": { "code": "482913", "createdAt": "2026-02-11T10:00:00Z" }
}
```

- 缺字段即默认值，所以旧版本写的文件仍能读。
- `deviceName` 与 `pendingPairing` 为空时不写进文件（`omitempty`）：空字段和「没设过」
  是同一件事，写出来只会让文件多两行噪音。
- `pendingPairing.code` 存**明文**。配对握手需要另一台设备显示的那个值本身；文件权限
  0600、目录 0700，配对码不进日志。若将来只需要校验摘要，改成存哈希即可，字段名不变。
  （这一条是推断，未与配对协议的实现核对过。）

## 行为契约

- `Load`：文件不存在**不是错误**，返回零值——首次运行就是「没有设置」。文件存在但
  读不出来或解析不了**是**错误，由调用方决定后果（应用按首次启动处理）。
- `Save`：先写同目录临时文件再 `rename`，崩溃不会留下半截文件被下次启动误读成首次
  运行；目录 0700、文件 0600。
- `PairingCode`：吃掉空格、连字符与 NBSP 后必须是恰好 6 位 ASCII 数字，返回规范化后
  的值；其余一律拒绝。粘贴来的分组形式（`482 913`）因此可以直接用。

## 边界

- 不依赖 MyGo、不联网，可以脱离界面单独测试。
- 不负责判断「该不该显示向导」：它只报告文件里的事实，由应用外壳决定（见 `gopaste`）。
- 不存界面偏好。语言、外观、删除前确认目前仍只在本次运行内有效；把它们搬进来是另
  一次改动，不是这个文件的既有职责。
