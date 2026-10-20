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
  },
  "retention": { "maxItems": 500 },
  "fileSyncMaxMB": 2048,
  "sync": {
    "enabled": false,
    "autoSend": false,
    "sound": "off",
    "files": {
      "enabled": false,
      "dir": "/Users/me/GoPasteFiles",
      "quotaMB": 4096,
      "retentionHours": 0,
      "autoClean": false
    }
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
- `retention` 存历史记录的自动清理上限（见 `gopaste`）。两个维度各自可缺：缺
  `maxItems` 或 `maxDays` 表示用户没选过这一维，应用继续用自己的默认值；写了 `0`
  表示明确的「不限」。字段用指针而不是 `int`，就是为了让这两种情况能分开——用
  `int` 时旧文件会读成「不限」，等于把所有老用户的保留策略静默关掉。
- 只有用户真改过的那一维才落盘，默认值不写进文件：将来调默认值，没选过的人跟得上。
- `fileSyncMaxMB` 是单个文件项发往另一台设备的体积上限（MiB）。缺省或小于 1 时
  取默认 2048；没有「不限」这一档，因为 0 意味着「什么都传不了」，不是一个可表达的
  选择。改回默认值时同样不写进文件。上限在界面上的位置与即时生效见 `gopaste`。
- `sync` 存同步页的全部选择：总开关（`enabled`）、自动发送（`autoSend`）、提示音
  （`sound`）与文件同步那一段（`files`）。默认值为「开」的开关用 `*bool`：缺字段就是开，
  写 `false` 才是用户关过；与 `ui.confirmDelete` 同一套写法。全是默认值时整个对象不写
  进文件。
  - `files.dir` 是收到的文件落盘的目录，空表示用应用自己的默认位置；改它只影响之后
    收到的文件，所以文件里只记当前选择，不记历史目录。
  - `files.quotaMB` 是收到的文件缓存上限（MiB），缺省或小于 1 时取默认 2048。没有
    「不限」这一档：0 意味着「什么都不留」，不是可表达的选择。
  - `files.retentionHours` 是收到的文件保留多少小时，缺省是默认 24；写了 `0` 是明确的
    「不限」。和 `retention` 的两个维度同一个理由：用 `int` 的话旧文件会被读成「不限」，
    等于把老用户的期限静默关掉。
  - `files.autoClean` 是自动清理开关，只决定过期文件要不要自动删；配额是上限，不受它
    影响（见 `gopaste`）。
  - `sound` 是提示音档位：`"off"`、`"sync"`（缺省）或 `"all"`。默认档不写进文件；
    认不出的取值由应用回退到 `"sync"`，与其它字段一样不让整份文件读失败。
- `fileSyncMaxMB` 与 `sync.files.quotaMB` 是两个不同的上限：前者管单个文件项，后者管
  收到的文件一共能占多少盘。两者都用「缺省即默认、默认不落盘」的写法。
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
- 不执行清理，也不测量磁盘：它只报告用户选了什么，配额与期限怎么落地见 `gopaste` 与
  `filescache`。
- 不存界面运行状态：窗口尺寸、侧栏展开、当前页面每次启动重新决定，不写进文件。
  `ui` 只记用户在设置页显式选过的偏好（语言、外观、删除前确认、缩放），不记这些偏好
  怎么被界面消费——那是 `gopaste` 的事。`retention` 不是界面偏好：它决定数据留多久，
  所以同样必须跨启动记住。
