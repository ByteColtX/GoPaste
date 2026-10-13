# GoPaste Design System

> Master rules for the GoPaste desktop application. Page-specific overrides belong
> in `design-system/gopaste/pages/` and must keep the rules below unless they
> document a deliberate exception.

**Status:** active
**Platform:** macOS, Windows, Linux
**UI stack:** Go + MyGo native UI
**Product:** local-first clipboard history and LAN device sync
**Mode:** Operate
**Design direction:** Minimalism & Swiss Style with native, flat controls

## Product principles

1. **Scan before reading.** Type, source, time, and sync state are visible at a glance.
2. **Copy is the primary action.** The selected history item keeps copy close and predictable.
3. **Keyboard first.** Search, navigation, list movement, selection, copy, and modal dismissal work without a pointer.
4. **Calm chrome.** Use one system accent, opaque surfaces, 1px dividers, and no decorative gradients.
5. **Trust through locality.** Make LAN connection, pairing, device status, and destructive scope explicit.

## Visual language

- Use a measured grid, 4 DIP spacing units, and predictable content widths.
- Keep the visual hierarchy in type, spacing, borders, and one system accent.
- Use the embedded Tabler SVG family for meaningful icons. Never use emoji or text glyphs as structural icons.
- Use color with a label, icon, or shape. Color never carries state by itself.
- Use opaque `theme.Surface` for sidebars, controls, and cards. Use alpha only for state fills and small indicators.
- Hover, pressed, selected, and focus states do not move surrounding content.
- Avoid gradients, glass effects, heavy shadows, and decorative motion.

## Color roles

All runtime colors come from `ui.Theme`, so system accent, high contrast, and light/dark appearance continue to work on every platform.

| Role | MyGo token | Usage |
| --- | --- | --- |
| Canvas | `theme.Background` | Window and page background |
| Surface | `theme.Surface` | Sidebars, controls, inputs |
| Elevated surface | `theme.Surface` | Cards, preview content, dialogs |
| Hover | `theme.SurfaceHover` | Hover and pressed state layers |
| Selected | `theme.Accent` (tinted) | Selected rows, filters, navigation; use `selectedFill` and `selectionBorder` |
| Border | `theme.Border` | Cards, controls, separators |
| Primary text | `theme.Text` | Headings and content |
| Secondary text | `theme.TextMuted` | Metadata, helper copy, timestamps |
| Accent | `theme.Accent` | Primary action, links, focus |
| Success | `theme.Success` | Online and completed states |
| Warning | `theme.Warning` | Favorites, waiting, attention |
| Destructive | `theme.Danger` | Clear, unpair, failure |
| Focus | `theme.Focus` | Keyboard focus ring |

Do not call `ui.Hex` from page code. Content-kind colors are derived from these semantic roles in one shared helper and receive a separate dark-mode treatment.

## Typography

Use MyGo's system font. It keeps Chinese, Latin, numbers, and platform controls aligned without a web font dependency.

| Role | Token | Weight | Use |
| --- | --- | ---: | --- |
| Page title | `fontPageTitle` | 600 | Detail page titles |
| Pane title | `fontPaneTitle` | 600 | History list and secondary sidebar titles |
| Section | `fontSection` | 600 | Empty-state and dialog titles |
| Subheading | `fontSubheading` | 600 | Settings group headings |
| Body | `fontBody` | 400 | Main content and descriptions |
| Row label | `fontBody` | 500 | Settings, device, and switch row titles |
| Control | `fontControl` | 500–600 | Buttons, fields, values |
| Metadata | `fontMeta` | 400 | Device and source metadata |
| Caption | `fontCaption` | 500 | Timestamps, compact tags |
| Code | `fontCode` | 400 | Clipboard code content |

User-facing text stays at or above the caption token. Long content wraps; truncation uses an ellipsis and a complete accessible label.

## Layout and spacing

The base spacing unit is `theme.Space(1)` (4 DIP by default). Use the shared tokens in `ui_tokens.go`:

- Inline: 8 DIP
- Control group: 8–12 DIP
- Section: 24 DIP
- Page top: 12 DIP below the title-bar strip
- Page side: 24 DIP on standard windows, 16–20 DIP in compact mode
- Page bottom: 28 DIP
- Control height: 36 DIP
- Compact control height: 32 DIP
- Card radius: 12 DIP
- Control radius: 8 DIP
- Dialog radius: 14 DIP

Responsive widths:

- Window minimum: 900 × 560 DIP.
- Compact navigation rail: 60 DIP.
- Expanded navigation rail: 176 DIP at 1200 DIP or wider.
- The compact rail widens to clear the window controls: it is at least as wide as the hidden title bar's left inset plus one spacing step, so the rail's trailing divider never crosses the macOS traffic lights (or a left-aligned window button row). The platform reports no inset on Windows and Linux, so the rail stays at its base width there.
- Secondary sidebar: 208 DIP in compact mode, 236 DIP otherwise.
- History list minimum: 300 DIP.
- History preview minimum: 460 DIP while split.
- Detail content maximum: 720 DIP.

At compact widths, controls stack before they clip. The history filter row may scroll horizontally; page text and actions do not.

## Component rules

### Navigation

- Keep History, Devices, and Settings in the same order on every platform.
- Keep the three destinations grouped together at the top of the rail; never anchor one to the bottom, so the current location always sits in the same region.
- Wide windows show icon and text. Compact windows keep a labeled, tooltip-enabled icon rail.
- Current location uses `Selected(true)`, `selectedFill(theme)`, and primary text.
- Navigation controls have stable keys, labels, tooltips, and visible focus rings.

### Buttons and controls

- Prefer MyGo semantic widgets: `ButtonBase`, `Checkbox`, `Switch`, `Select`, `Segmented`, `List`, and `Modal`.
- Use shared control heights and radii; do not add page-specific control sizes.
- Filled accent controls are reserved for the single primary action in a context.
- Secondary actions stay quiet. Destructive actions use `theme.Danger` and confirmation.
- Preserve the default MyGo focus ring or draw an equivalent ring without changing layout bounds.

### Cards and dividers

- Cards use `standardCard`, a 1px semantic border, and an opaque surface.
- Dividers use `standardDivider` and separate sibling rows.
- Selection keeps a stable border so content does not jump between states.

### States

Every interactive control supports default, hover, pressed, focused, selected, and disabled states. Each page defines empty, loading, success, warning, error, offline, and unavailable states where those states can occur.

Errors name the cause and provide a recovery action. Loading feedback reserves the final layout and avoids flashing for near-instant work. Toasts contain status only and never clipboard content.

### History

- A list row is two lines: a rounded, kind-tinted icon tile, then the content title with its favorite star, then a metadata line of type and time. The tile icon and color follow the item's own kind, so text, link, code, image, and file rows are told apart by icon before the label is read.
- Show type and favorite state in the list metadata row; show type, size, source device, sync state, and time in the preview metadata row, with the source device and its sync chip sitting before the time. The source device reads 本机 for items captured here and names the paired device for items that arrived over the LAN.
- Keep selected list row and preview synchronized by stable item ID.
- Show the automatic content tag (链接, 代码) on the preview header's second line so it never takes its own line. The list row carries the same subtype through its tile icon and colored metadata label instead of a separate tag line.
- Keep the preview's item actions in a floating bar pinned to the bottom of the preview pane and centered horizontally over the content column: delete (destructive) sits on the left, separated by a divider from favorite and copy (the primary action, on the right). Let it float over scrolled content while the body reserves matching bottom space, so the last line never hides behind it.
- Confirm deleting one record the same way as clearing all history, so no history action removes saved data in a single step.
- Show sync state only for items captured on this machine. An item received from a paired device already arrived here, so it shows no sync status.
- Show sync state as a compact chip in the preview metadata row, set beside the source device so the record's origin and its delivery read as one group: synced, partly synced, peer offline, or replaced. A partly synced item keeps the success visible instead of reading as a plain peer-offline failure. The chip costs no space while idle.
- Open the sync chip's popover on click, not on hover: the chip sits above the floating preview action bar, so a hover-opened popover misfires when the pointer crosses it on the way to the buttons.
- Keep the preview metadata row at one fixed height. An item captured here shows a sync chip and one that arrived over the LAN does not, so the row must not change height or the content below it jumps when the selection changes.
- Reveal delivery detail in a popover opened by clicking the chip: a batch resend, then each paired device's outcome with a per-device resend while the item is still waiting. A replaced item, never delivered to any device, adds one line of explanation and the same per-device resend list.
- Distinguish first-use empty, search empty, filter empty, and unavailable preview.

### Devices and settings

- Use the shared secondary sidebar and detail width.
- Pair status text with a status dot or icon; never use color alone.
- Make the direct, peer-to-peer connection explicit in device details and pairing states, without naming a specific network type.
- Use helper text for switches and direction controls so disabled states explain their scope.
- Destructive actions confirm the affected data or device.

## Interaction and accessibility

- History supports search focus, Up/Down, Home/End, Enter to copy, and Escape.
- Tab order follows visual order: global navigation, page controls, content, actions.
- Custom selected and disclosure controls expose `Selected` or `Expanded` state.
- Dialogs trap focus, close with Escape, and restore focus to the opener.
- Use `c.Preferences()` for `ReduceMotion`, `HighContrast`, and `TextScale` behavior.
- Maintain at least 4.5:1 contrast for normal text and 3:1 for meaningful icons and borders.
- Disabled controls are semantically disabled, visually muted, and explained when scope matters.

## Motion and feedback

- Use MyGo `Animate` and `Loop`; no third-party animation package.
- Pressed feedback is visible within 100–150 ms.
- Disclosure and modal transitions are short and interruptible.
- Skeletons and spinners run only for meaningful waits and stop when content is ready.
- When `ReduceMotion` is enabled, render the final state immediately and avoid decorative loops.

## Privacy and platform invariants

- Synchronization remains limited to paired devices on the local network.
- Do not add relay, cloud, third-party proxy, or non-LAN fallback connections.
- Clipboard payloads never enter ordinary logs, telemetry, or third-party services.
- Platform-specific behavior stays isolated so macOS, Windows, and Linux builds retain their existing behavior.

## Implementation map

- `ui_tokens.go` owns shared metrics, semantic colors, and primitives.
- `view.go` owns the shell, navigation, history list, preview, and history actions.
- `device_view.go` owns device navigation, sync controls, pairing, and connection details.
- `settings_view.go` owns settings navigation and preference groups.
- `icons.go` and `resources/icons/` own the embedded icon family.
- `model.go` and `state.go` own stable keys, page state, and domain values.

## Pre-delivery checklist

- [ ] No emoji or text glyph is used as a structural icon.
- [ ] No page code contains raw color literals.
- [ ] Light, dark, and high-contrast themes preserve text, border, status, and focus contrast.
- [ ] Focus is visible on navigation, lists, fields, selects, switches, and modal actions.
- [ ] History filters remain usable at the 300 DIP list minimum.
- [ ] Empty, loading, success, warning, error, offline, and unavailable states have recovery copy where applicable.
- [ ] Chinese, English, and enlarged system text do not clip controls.
- [ ] No clipboard content enters ordinary logs, telemetry, cloud services, or relays.
- [ ] Platform-specific behavior remains isolated and other platform builds remain intact.
