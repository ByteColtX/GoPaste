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
| Selected | `theme.Accent` (tinted) | Selected rows, filters, navigation, dropdown items, and the chosen segment; use `selectedFill` and `selectionBorder` |
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
- Page top: 12 DIP below the title-bar band. Every pane — the sidebar, the history list, the preview, and the device and settings details — reserves the band and then starts its first line at this one inset, so all columns share a single first-line baseline in both sidebar states.
- Page side: 24 DIP on standard windows, 16–20 DIP in compact mode
- Page bottom: 28 DIP
- Control height: 36 DIP
- Compact control height: 32 DIP
- Control radius: 8 DIP
- Card radius: 12 DIP

Two radii cover the whole app: `controlRadius` for controls, rows, navigation and filter items, tags, and chips; `cardRadius` for cards, popovers, dialogs, floating bars, large tiles, and media containers. No page adds a third value. A nested surface derives its radius from its parent (outer radius minus the inset), so a dropdown panel at `cardRadius` holds options at `controlRadius`.

Responsive widths:

- Window minimum: 1000 × 560 DIP.
- Sidebar: 208 DIP when expanded, collapsible to 0 DIP. It is the only sidebar in the window.
- History list minimum: 300 DIP.
- History preview minimum: 460 DIP while split.
- Detail content maximum: 720 DIP.

At compact widths, controls stack before they clip. The sidebar scrolls vertically; page text and actions do not.

## Component rules

### Navigation

- One sidebar serves the whole window, and its content follows the current section: History shows the filter groups, Devices shows the device list, Settings shows the settings panes. A page never adds a second sidebar of its own, so the leading column never shows one section's controls while another section is displayed.
- The sidebar carries no title line of its own: the switcher's selected item names the section. The history search field is its first item and appears only on History.
- Each section's sidebar body carries its own key, so scroll position never carries across sections.
- The section switcher sits at the bottom of the sidebar: icon-only History, Settings, and Devices at the left, the light/dark appearance toggle at the right, with labels in tooltips and accessible names. History must be present, because the contextual sidebar shows no filters on the other sections, so it is the only pointer route back.
- Known trade-off: HIG › Sidebars advises against critical actions at a sidebar's bottom edge, because a relocated window can hide it. The switcher stays there by product decision.
- The history list header reads 历史记录, and the sidebar has no second title beside it.
- The device list's refresh and add-device actions sit in the list header row, beside the list they act on; the sidebar's bottom holds only the informational online count.
- The sidebar collapses by animating its width to zero behind a fixed-width, clipped content column, so its content slides out of view instead of squeezing. The detail panes slide in step with the same spring, and their height never changes: only the leftmost pane's first row shifts right to clear the window controls.
- The appearance toggle cycles the three modes in the settings pane's order — follow system, light, dark — and shows the mode a click lands on: a half-filled circle for follow-system, a sun for light, a moon for dark. The settings pane's segmented control stays the way to pick one explicitly.
- A collapse/expand button sits in the title bar, right of the platform window controls and 12 DIP clear of them. HIG › Accessibility asks for enough padding between controls to reduce wrong-control clicks: about 12 DIP around a bezelled control, about 24 DIP around an unbezelled one. Collapsing hides the sidebar entirely; the button stays, so it can return.
- Filter rows are left-aligned: the icon sits at the sidebar's content edge, the label follows it, and section labels share that same left edge.
- Current destination and current filter both use `Selected(true)`, `selectedFill(theme)`, and primary text.
- Navigation and filter controls have stable keys, labels, tooltips, and visible focus rings.
- Choosing any filter returns to History with that filter applied.

### Search

- The history search field sits below the section switcher, above the filter list.
- Typing in the search from another page returns to History, so the result is visible immediately.
- The search shortcut (Cmd+K, Cmd+F) expands the sidebar before focusing the field.

### Buttons and controls

- Prefer MyGo semantic widgets: `ButtonBase`, `Checkbox`, `Switch`, `Select`, `Segmented`, `List`, and `Modal`.
- Use shared control heights and radii; do not add page-specific control sizes.
- Filled accent controls are reserved for the single primary action in a context.
- Secondary actions stay quiet. Destructive actions use `theme.Danger` and confirmation.
- Preserve the default MyGo focus ring or draw an equivalent ring without changing layout bounds.

### Cards and dividers

- Cards use `standardCard`, a 1px semantic border, and an opaque surface.
- Cards, popovers, dialogs, and large media containers share the card radius; controls, rows, and navigation items share the control radius.
- Dividers use `standardDivider` and separate sibling rows.
- Selection is one shared style everywhere: a light accent wash with no visible outline. The outline stays transparent unless high contrast asks for one, and it always keeps its 1px slot so content does not jump between states.
- Every selectable row, filter, navigation item, dropdown option, and segmented segment uses `selectedFill` and `selectionBorder`, so no control introduces a brighter highlight of its own.

### States

Every interactive control supports default, hover, pressed, focused, selected, and disabled states. Each page defines empty, loading, success, warning, error, offline, and unavailable states where those states can occur.

Errors name the cause and provide a recovery action. Loading feedback reserves the final layout and avoids flashing for near-instant work. Toasts contain status only and never clipboard content.

### History

- The sidebar groups history filters top to bottom: All and Favorites, then Type (text, rich text, image, file), Tag (link, code), Source (all sources, then each paired device), and Time (all time, today, yesterday, last 7 days, last 30 days). The Tag group holds only the detected subtypes, so it never repeats a Type: an image item is reached through Type, not through a second Image row.
- Filter section labels sit 16 DIP below the previous group and 8 DIP above their own rows, so a label reads as part of the group it names. The values follow Apple's 8 pt baseline grid, where 8 DIP is the standard label-to-control gap and section separation is the larger step.
- Every pane reserves the title-bar band, so its content keeps a clear margin from the window's top edge and never sits under the window controls. The band holds the window controls and the sidebar toggle, centred on one midline, and each pane's header row doubles as the drag region.
- Because the content starts below the band, collapsing the sidebar needs no horizontal or vertical dodge: the columns simply change width, and every first line, header height, and row position stays put.
- Collapsing the sidebar must never move content vertically: the band and the first-line inset are the same in both states, so nothing jumps and no pane loses room. The band and the inset follow Apple's unified toolbar, where the controls and the inline title share the toolbar row, and the HIG-derived content margin of 12–14 DIP below the toolbar before the first control.
- Type filters by payload format, so Text includes links and code; Tag narrows to a detected subtype. Choosing the selected facet again clears it, and All clears every facet.
- A list row is two lines: a rounded, kind-tinted icon tile, then the content title with its favorite star, then a metadata line of type and time. The tile icon and color follow the item's own kind, so text, link, code, image, and file rows are told apart by icon before the label is read.
- Show type and favorite state in the list metadata row; show type, size, source device, sync state, and time in the preview metadata row, with the source device and its sync chip sitting before the time. The source device reads 本机 for items captured here and names the paired device for items that arrived over the LAN.
- Name the detected language after the type in the list metadata row for code items (代码 · Go), so the list tells code apart by language without a separate tag; omit it when the language is not recognised, leaving the type alone.
- Keep selected list row and preview synchronized by stable item ID.
- Show the automatic content tag (链接, 代码) on the preview header's first line, beside the type label, so the type and its subtype read together and the tag never takes its own line. The list row carries the same subtype through its tile icon and colored metadata label instead of a separate tag line.
- Keep the preview's item actions in a floating bar pinned to the bottom of the preview pane and centered horizontally over the content column: delete (destructive) sits on the left, separated by a divider from favorite and copy (the primary action, on the right). Let it float over scrolled content while the body reserves matching bottom space, so the last line never hides behind it.
- Confirm deleting one record the same way as clearing all history, so no history action removes saved data in a single step.
- Show sync state only for items captured on this machine. An item received from a paired device already arrived here, so it shows no sync status.
- Show sync state as a compact chip in the preview metadata row, set beside the source device so the record's origin and its delivery read as one group: synced, partly synced, peer offline, or replaced. A partly synced item keeps the success visible instead of reading as a plain peer-offline failure. The chip costs no space while idle.
- Open the sync chip's popover on hover: resting the pointer on the chip, or on the popover itself, keeps the delivery detail open so the pointer can move between them, and it closes once the pointer leaves both.
- Keep the preview metadata row at one fixed height. An item captured here shows a sync chip and one that arrived over the LAN does not, so the row must not change height or the content below it jumps when the selection changes.
- Reveal delivery detail in a popover opened by hovering the chip: a batch resend, then each paired device's outcome with a per-device resend while the item is still waiting. A replaced item, never delivered to any device, adds one line of explanation and the same per-device resend list.
- Distinguish first-use empty, search empty, filter empty, and unavailable preview.

### Devices and settings

- The device list and the settings panes live in the shared sidebar, not in a second column beside it; the detail pane takes the remaining width.
- Pair status text with a status dot or icon; never use color alone.
- Make the direct, peer-to-peer connection explicit in device details and pairing states, without naming a specific network type.
- Use helper text for switches and direction controls so disabled states explain their scope.
- Destructive actions confirm the affected data or device.
- Let a device unpairing be undone for a short window: the toast offers Undo, which puts the device back in its place and reselects it, and the chance ends when the toast times out.

## Interaction and accessibility

- History supports search focus, Up/Down, Home/End, Enter to copy, and Escape.
- Tab order follows visual order: global navigation, page controls, content, actions.
- Custom selected and disclosure controls expose `Selected` or `Expanded` state.
- Dialogs trap focus, close with Escape, and restore focus to the opener.
- Use `c.Preferences()` for `ReduceMotion`, `HighContrast`, and `TextScale` behavior.
- Maintain at least 4.5:1 contrast for normal text and 3:1 for meaningful icons and borders.
- Disabled controls are semantically disabled, visually muted, and explained when scope matters.

## Motion and feedback

- Use MyGo `ElementTransition`, `Animate*`, `AnimatePose` and `Loop`; no third-party animation package. The shared motion tokens live in `ui_tokens.go`.
- Follow Apple's spring guidance (WWDC23, "Animate with springs"): move things with springs, so a change on the way keeps its velocity and a second click reverses smoothly instead of stopping dead. Use a zero-bounce spring for anything clipped while it moves, such as the sidebar, so overshoot cannot show as a gap.
- Keep durations inside NN/g's 100–500 ms band and prefer the shortest that still reads: 120 ms for hover and selection color, 140 ms for a page cross-fade, 300 ms for layout springs.
- Animate only what changed: `motionColors` for color feedback, `motionLayout` and `motionSlide` for layout springs, `motionAppear` for content entering, `motionPage` for a page change.
- Fade when movement carries no meaning (HIG › Motion), as the page cross-fade does.
- Pressed feedback is visible within 100–150 ms.
- Skeletons and spinners run only for meaningful waits and stop when content is ready.
- MyGo goes straight to the target when `ReduceMotion` is on, so transitions need no extra branch.

## Privacy and platform invariants

- Synchronization remains limited to paired devices on the local network.
- Do not add relay, cloud, third-party proxy, or non-LAN fallback connections.
- Clipboard payloads never enter ordinary logs, telemetry, or third-party services.
- Platform-specific behavior stays isolated so macOS, Windows, and Linux builds retain their existing behavior.

## Implementation map

- `ui_tokens.go` owns shared metrics, semantic colors, and primitives.
- `view.go` owns the shell, the contextual sidebar, its section switcher, the header search, the history list, the preview, and the history actions.
- `device_view.go` owns device navigation, sync controls, pairing, and connection details.
- `settings_view.go` owns settings navigation and preference groups.
- `icons.go` and `resources/icons/` own the embedded icon family.
- `model.go` and `state.go` own stable keys, page state, and domain values.

## Pre-delivery checklist

- [ ] No emoji or text glyph is used as a structural icon.
- [ ] No page code contains raw color literals.
- [ ] Light, dark, and high-contrast themes preserve text, border, status, and focus contrast.
- [ ] Focus is visible on navigation, lists, fields, selects, switches, and modal actions.
- [ ] The sidebar stays usable at the 300 DIP list minimum and the 1000 DIP window minimum, and collapses to 0 without clipping the content.
- [ ] Empty, loading, success, warning, error, offline, and unavailable states have recovery copy where applicable.
- [ ] Chinese, English, and enlarged system text do not clip controls.
- [ ] No clipboard content enters ordinary logs, telemetry, cloud services, or relays.
- [ ] Platform-specific behavior remains isolated and other platform builds remain intact.
