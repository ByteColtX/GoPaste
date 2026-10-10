# GoPaste Design System

> Master rules for the GoPaste desktop application. Page-specific overrides belong in
> `design-system/gopaste/pages/` and must keep the rules below unless they document a
> deliberate exception.

**Status:** active
**Platform:** macOS, Windows, Linux
**UI stack:** Go + MyGo native UI
**Product:** local-first clipboard history and LAN device sync
**Design direction:** Minimalism & Swiss Style with flat, functional controls

This system is adapted from the verified `ui-ux-pro-max` recommendations for
desktop productivity tools. Web-only patterns such as CSS, cursor states, and
scroll reveal animation do not apply to GoPaste's native surface.

## Product principles

1. **Scan before reading.** Type, source, time, and sync state should be visible
   at a glance.
2. **Copy is the primary action.** The selected history item and its preview keep
   the copy action close and predictable.
3. **Keyboard first.** Search, list movement, selection, and copy must work without
   a pointer. Focus indicators remain visible.
4. **Calm chrome.** Use one accent, quiet surfaces, 1px dividers, and no
   decorative gradients or heavy shadows.
5. **Trust through locality.** Keep sensitive clipboard data on the local device;
   use explicit labels for LAN connection, device status, and destructive actions.

## Visual language

### Style

- Minimal, grid-based layouts with generous but consistent whitespace.
- Flat surfaces with a small amount of tonal separation; avoid glass effects,
  gradients, and ornamental decoration.
- Use the embedded Tabler SVG family for meaningful icons. Do not use emoji or
  text glyphs as structural icons.
- Use color to reinforce type and state, never as the only state signal.
- Hover and pressed states change fill or opacity without moving layout bounds.

### Color roles

All runtime colors come from `ui.Theme` so the native light/dark appearance and
platform contrast remain intact. The hex values are reference fallbacks for
design review, not per-screen literals.

| Role | MyGo token | Light reference | Dark reference | Usage |
| --- | --- | --- | --- | --- |
| Window background | `theme.Background` | `#F5F5F7` | `#181A1F` | Root window and detail panes |
| Surface | `theme.Surface` | `#F4F4F5` | `#27272A` | Inputs, sidebars, controls |
| Panel | `theme.Surface` + alpha | `#FFFFFF` | `#2B2E35` | Cards and preview blocks |
| Primary text | `theme.Text` | `#18181B` | `#F4F4F5` | Headings and content |
| Secondary text | `theme.TextMuted` | `#52525B` | `#A1A1AA` | Metadata and helper copy |
| Border | `theme.Border` | `#D9D9DE` | `#3F3F46` | Card and control boundaries |
| Accent | `theme.Accent` | `#2563EB` | `#3B82F6` | Selection, links, active controls |
| Focus | `theme.Focus` | accent at 55% | accent at 60% | Keyboard focus ring |
| Success | `theme.Success` | `#16A34A` | `#22C55E` | Online and enabled states |
| Warning | `theme.Warning` | `#D97706` | `#F59E0B` | Favorites and attention |
| Destructive | `theme.Danger` | `#DC2626` | `#EF4444` | Clear, unpair, and other data loss |

The product palette search also identified teal `#0D9488` with orange `#EA580C`
as a useful future brand direction. If GoPaste adopts a custom theme later, map
those values to `Accent` and `Warning` in one theme definition instead of adding
screen-level hex values.

### Typography

Use MyGo's system font. It follows the host platform and keeps Chinese and Latin
text readable without downloading a web font. The current token scale is the
source of truth:

| Role | Token | Scale | Weight | Use |
| --- | --- | ---: | ---: | --- |
| Page title | `fontPageTitle` | 1.75 rem | 600 | Detail page titles |
| Section title | `fontSection` | 1.125 rem | 600 | Pane and modal titles |
| Subheading | `fontSubheading` | 0.9375 rem | 600 | Settings groups |
| Body | `fontBody` | 1 rem | 400 | Main content and descriptions |
| Control | `fontControl` | 0.875 rem | 400–600 | Buttons and field values |
| Metadata | `fontMeta` | 0.8125 rem | 400 | Device and source metadata |
| Caption | `fontCaption` | 0.75 rem | 400–600 | Timestamps and compact labels |
| Code | `fontCode` | 0.9375 rem | 400 | Monospace clipboard content |

Do not introduce text below the caption scale for user-facing content. Keep body
text at or above the platform's readable default size.

## Layout and spacing

Use the MyGo spacing unit (4 DIP by default) and the shared tokens in
`ui_tokens.go`. Keep the 4/8 rhythm for controls and the larger 16/24/32 rhythm
for page sections.

| Token | Value in spacing units | Use |
| --- | ---: | --- |
| `spaceInline` | 2 | Icon-to-label and compact gaps |
| `spaceControl` | 3 | Control group gaps |
| `spaceSection` | 6 | Page section gaps |
| `spacePageSide` | 8 | Detail pane gutters |
| `spacePageTop` | 10 | Detail pane top inset |
| `spacePageBottom` | 12 | Detail pane bottom inset |

Desktop frame constraints:

- Window minimum: 900 × 560 DIP.
- Navigation rail: 64 DIP.
- Secondary sidebar: 224 DIP.
- History list minimum: 300 DIP.
- Preview minimum: 460 DIP.
- Detail content maximum: 720 DIP.

The history filter row may scroll horizontally when the list pane is narrow. Do
not allow controls or text to be clipped by a fixed-width row.

## Component rules

### Navigation

- The 64 DIP rail contains history, devices, and settings in that order.
- Icon-only controls must have a stable key, accessible `Label`, and tooltip.
- The selected item uses `selectedFill(theme)` and primary text; hover uses
  `hoverFill(theme)` without changing size.

### Buttons and controls

- Use `ui.ButtonBase` for custom buttons so MyGo supplies keyboard focus, Enter,
  and Space behavior.
- Use shared control heights (`controlHeight` and `compactHeight`) and radii;
  do not add one-off sizes in page code.
- Keep icon buttons at least one control-height square and give them an explicit
  label or tooltip.
- Use filled accent controls only for the primary action in a context. Secondary
  actions stay quiet and destructive actions use `theme.Danger`.
- Preserve MyGo's default focus ring. Never hide focus without drawing an
  equivalent ring.

### Cards and dividers

- Cards use `standardCard`, a 1px semantic border, and a low-contrast panel fill.
- Dividers use `standardDivider` and separate sibling rows; do not use shadows to
  imply hierarchy.
- Selected history rows keep a transparent border when idle so selection does not
  shift text or icons.

### History

- Show type, favorite state, source, and time in the metadata row.
- Keep the selected row and preview synchronized.
- The preview header keeps copy and favorite actions within the same visual group.
- Empty states explain the next action in one short sentence.

### Devices and settings

- Use the shared secondary-sidebar width and detail-page max width.
- Pair status text with a status dot; never use color alone for online/offline.
- Destructive actions require a confirmation modal and name the affected data or
  device.
- Labels describe what a switch or select changes, with helper text for scope.

## Interaction and accessibility

- Keyboard order follows visual order. History supports Up/Down, Home/End, and
  Enter to copy; Escape closes overlays and clears search when applicable.
- Every control has a semantic label. Decorative icons beside visible labels do
  not carry additional meaning.
- Focus must remain visible in both themes and inside confirmation dialogs.
- Maintain at least 4.5:1 contrast for normal text and 3:1 for meaningful icons,
  borders, and state indicators.
- Disabled controls are visibly muted and do not accept input.
- Toasts confirm copy, favorite, sync, and destructive outcomes without logging
  clipboard contents.

## Motion and feedback

GoPaste is a fast utility, so the default is static layout with immediate native
feedback. Use MyGo's built-in pressed and focus states. If a transition is added,
keep it short (150–250 ms), avoid layout-changing transforms, and honor reduced
motion settings when the platform exposes them. Do not add scroll-reveal or
autoplay motion to the desktop utility.

## Page map

| Page | Primary job | Key components |
| --- | --- | --- |
| History | Find and copy a previous clipboard item | Search, filters, list, preview |
| Devices | Pair and control LAN peers | Device list, sync directions, connection info |
| Settings | Change local preferences | Secondary navigation, grouped settings, confirmation |

## Implementation map

- `ui_tokens.go` owns spacing, typography, radius, and semantic color helpers.
- `view.go` owns navigation, history list, preview, and history actions.
- `device_view.go` owns device and sync controls.
- `settings_view.go` owns preference groups and settings navigation.
- `icons.go` and `resources/icons/` own the embedded SVG icon family.
- `model.go` owns stable keys, page state, and domain labels; UI elements do not
  live in persistent state.

## Pre-delivery checklist

- [ ] No emoji or text glyph is used as a structural icon.
- [ ] All icon-only controls have labels and tooltips.
- [ ] Light and dark themes preserve text, border, focus, and status contrast.
- [ ] Focus is visible on navigation, lists, fields, selects, switches, and modal actions.
- [ ] History filters remain usable at the 300 DIP list minimum.
- [ ] Destructive actions confirm scope before changing local data or pairings.
- [ ] No clipboard content enters ordinary logs, telemetry, cloud services, or relays.
- [ ] Platform-specific behavior remains isolated and the other platform builds are unaffected.
