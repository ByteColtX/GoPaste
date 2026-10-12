# 图标来源

GoPaste 把选中的 SVG 图标随 Go 二进制一起嵌入（见 `icons.go` 的 `//go:embed`）。
所有图标都在 24px 网格上，运行时通过 `currentColor` 取色，随浅色/深色主题自动适配。

## Tabler Icons（描边风格）

- `brand-apple.svg`、`brand-windows.svg`、`brand-ubuntu.svg`，以及其余界面图标。
- 来源：<https://github.com/tabler/tabler-icons>
- 许可：MIT，全文见同目录 `LICENSE`。

## Simple Icons（实心风格）

- `brand-linux.svg`：Linux 的 Tux 企鹅。Tabler 没有企鹅或通用 Linux 图标，因此这一枚取自 Simple Icons。
- 来源：<https://github.com/simple-icons/simple-icons>
- 许可：CC0 1.0 Universal，全文见同目录 `LICENSE-CC0.md`。

> 注意：`brand-linux.svg` 是实心剪影，与其余描边图标风格不同，属于有意的例外。
