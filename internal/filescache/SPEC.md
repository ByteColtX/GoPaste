---
id: filescache
type: submodule-design
parent: gopaste
title: 文件缓存清理规则（internal/filescache）
status: draft
tags:
  - file-sync
  - retention
---

# 文件缓存清理规则

## 职责

决定「哪些收到的文件该删」：一条保留期限，一条体积配额。它只读目录算体积，不删任何
东西——删记录与删目录都由应用外壳做，历史记录的唯一真相不在这里。

只依赖标准库，不碰 MyGo、不联网，可以脱离界面单独测试。

## 契约

- `Entry{ID, Dir, Bytes, ReceivedAt}`：一条收到的文件记录。`Bytes` 由 `Measure` 从
  `Dir` 算出；目录不在了按 0 算——一块显示与清理用的数字不该因为刚好被删掉的记录变成
  错误态。
- `Policy{QuotaBytes, MaxAge}`：两个上限都可以关掉，`MaxAge` 不大于 0 表示不过期，
  `QuotaBytes` 不大于 0 表示不限量。
- `Plan(entries, policy, now)` 返回要删的 ID，最旧的在前：
  1. 先挑出 `now-ReceivedAt > MaxAge` 的条目；
  2. 剩下的按最旧优先删，直到总量回到配额以内。
- **过期条目先出局且不计入配额总量**：它们本来就该走，把它们算进总量会让一条过期的
  大文件连带删掉还在期限内的记录。
- 同一到达时刻按内容 ID 排序：同一份缓存永远得到同一份计划，测试与排查都不必猜顺序。
- `DirectoryBytes` 只累加普通文件，不跟随符号链接：链接指向的东西不在这棵目录里，算
  进来会让数字与目录的实际体积对不上。

## 边界

- 不认历史记录、不认内容类型：调用方把「收到的文件条目」投影成 `Entry` 再交进来。
- 不删文件、不删记录、不落盘：`Plan` 只给 ID，执行归 `gopaste`。
- 不决定什么时候跑：触发点（启动、收到文件、改设置、手动清理）在 `gopaste`。
