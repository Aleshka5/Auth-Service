# Service design language

Status: **extracted** from the shipped hub (`web/`). This is the contract a new Filenkov host follows so it looks like the others.
Scope: the face of a service — color, type, chrome, controls, page states. Auth, gateway routing, and roles stay in [AGENTS.md](../AGENTS.md) and [docs/auth-gateway.md](auth-gateway.md). The resume tree stays in [docs/resume-tree-frontend.md](resume-tree-frontend.md).
Audience: whoever builds the next host (storage, messenger, image, chat, or one that does not exist yet).

There is no shared UI package. The hub is the reference. A new service copies the files named in §12 and keeps the rules below. When a token changes, it changes on the hub first, and the same diff is ported.

---

## 1. What to save

These are the solutions that make two hosts feel like one product. Save them. Build the service's own screens on top.

| Save | Where it lives today | What a new service does with it |
|---|---|---|
| Paper palette and the two accent inks | `web/src/styles/tokens.css` | Copy the file. Use the tokens. Add a token on the hub before inventing a hex. |
| Type scale, measure, 4px space, two motion durations | same file | Every size, gap, and duration comes from the scale. |
| Fraunces + Public Sans, self-hosted, with metric-matched fallbacks | `web/src/styles/fonts.css`, `web/src/styles/base.css`, `web/public/fonts/` | Copy the faces and the `@font-face` rules. Preload the two latin files. |
| Flat paper, one grain layer, copper selection, focus ring | `web/src/styles/base.css` | Copy the file. |
| Diamond mark, wordmark, fixed header, service links, sign-in, footer, skip link | `web/src/components/Layout.tsx`, `web/src/styles/shell.css` | Copy the chrome. Point the wordmark at `https://filenkov.store`. Mark the current host in the nav. |
| Header that condenses and hides on scroll without re-rendering | `web/src/hooks/useScrollChrome.ts` | Copy the header half of the hook on any page that scrolls. |
| Buttons, fields, banners, tables, empty states | `web/src/styles/forms.css`, `web/src/pages/Auth.tsx`, `web/src/pages/Admin.tsx` | Copy `forms.css`. Compose pages from those classes. |
| Article vocabulary: kicker, lede, section, chips, figure, callout, metric, timeline, definition list | `web/src/styles/article.css`, `web/src/components/article/index.tsx` | Copy when the service has prose. A tool with no prose can skip this file. |
| Hairline rows for anything the reader chooses next | `web/src/styles/tree.css` (`.branch-card` only) | Reuse the row, the index number, and the accent bar. Leave the spine behind. |
| Page states: checking sign-in, delayed skeleton, banner error, empty line | `Auth.tsx`, `Admin.tsx`, `web/src/hooks/useDelayedFlag.ts` | Same sentences, same delay, same colors. |
| Light scheme, English, one `h1`, skip link, 44px targets | `web/index.html`, `Layout.tsx` | Copy the constraints. |

## 2. What stays on the hub

These are the resume, and they are why the hub looks like a magazine. A storage app or a chat host that grows them will stop looking like a tool and start looking like a second resume.

| Stays on the hub | Why |
|---|---|
| Resume registry, node folders, spine, branch-card *content* | The tree is the hub's information architecture. A service has its own. |
| The owl, the greeting, the docked back-to-top, the read ring | Mascot of the resume. `shell.css` from the comment `the owl` downward is hub-only. |
| Owl chat (`web/src/styles/chat.css`) | Rounded corners (5–16px) and the only drop shadows in the product. That treatment belongs to this one panel. A new floating surface uses `.panel` (§7). |
| `NodeView`, prefetch, sitemap-from-folders | Hub routing. A service has its own routes. |
| The auth page itself (`/auth`, the Google button) | One sign-in, on the hub. Other hosts send the browser there (§9). |
| The inline `fontSize: '2.1rem'` on the auth `h1` | A one-off. New pages use `--step-4`. |

---

## 3. Principle

A Filenkov screen is a sheet of warm paper with one column of text, hairline rules, and a single copper accent. Boxes, shadows, and extra colors have to earn a place, and almost none have.

When a layout wants a card, draw a rule. When it wants a second accent, use ink, muted ink, or danger. When it wants a shadow, it is the owl chat, and that file is already written.

The page is light. `web/index.html` sets `color-scheme: light`. A service does not ship a dark theme of its own.

The language is English.

---

## 4. Tokens

Copy `web/src/styles/tokens.css` unchanged. The values below are what that file ships. Contrast ratios are WCAG against `--paper`. Two accent tokens exist because `#b85c38` on `#f4efe6` is 4.0:1 and fails AA for text.

| Token | Value | Use |
|---|---|---|
| `--paper` | `#f4efe6` | Page background |
| `--paper-raised` | `#fffdf8` | The raised surface: figures, the auth panel, tables |
| `--ink` | `#1a1612` | Body text, primary buttons |
| `--ink-muted` | `#5c5348` | Secondary text. 6.6:1 on paper |
| `--rule` | `#d9d1c3` | Hairlines, field borders, skeleton bars |
| `--rule-strong` | `#b9ae9c` | A spine or an active rule, when a hairline is too quiet |
| `--accent` | `#b85c38` | Marks only: dots, bars, icons, selection, strokes at least 3px. 4.0:1 on paper, so it is never text |
| `--accent-text` | `#8f4128` | Any copper text or text-sized icon. 6.2:1 on paper |
| `--chip` | `#e6ddd0` | Chips, table headers, ghost-button hover, neutral banners |
| `--danger` | `#8c2f24` | Errors, `BLOCKED`, destructive buttons |

| Token | Value | Use |
|---|---|---|
| `--serif` | Fraunces, then Fraunces Fallback, Iowan Old Style, Palatino Linotype | Headings, wordmark, big numbers |
| `--sans` | Public Sans, then Public Sans Fallback, system-ui, Segoe UI | Body, UI, labels |
| `--step--1` | `0.8125rem` | Meta, nav links, chips, captions, table text nearby |
| `--step-0` | `1.0625rem` | Body. Also the root `font-size` |
| `--step-1` | `1.25rem` | Lede, row titles |
| `--step-2` | `1.5rem` | `h3`, metric values |
| `--step-3` | `1.9rem` | `h2` |
| `--step-4` | `clamp(2.2rem, 4.5vw, 3.1rem)` | `h1` |
| `--measure` | `34rem` | Reading column, about 66 characters at body size |

Space is a 4px scale. A margin, padding, or gap in a service stylesheet is one of these:

`--s1` 0.25rem · `--s2` 0.5rem · `--s3` 0.75rem · `--s4` 1rem · `--s5` 1.5rem · `--s6` 2rem · `--s7` 3rem · `--s8` 4.5rem

Motion is two durations and one curve:

| Token | Value |
|---|---|
| `--dur-fast` | `140ms` — hover, small fades |
| `--dur` | `240ms` — header, page entrance |
| `--ease` | `cubic-bezier(0.2, 0, 0, 1)` |

`prefers-reduced-motion: reduce` sets both durations to `1ms` inside `tokens.css`. Copy that block with the file. Scroll-into-view uses `behavior: 'auto'` under the same media query.

Chrome heights: `--header-h` is `4.5rem` (top of page), `--header-h-c` is `3.5rem` (condensed). The shell's top padding is the expanded height, because the header is fixed.

`--read` is the owl's progress ring. Leave it in the file so the token set stays one copy. A service does not paint the ring.

### 4.1 The one color that is still a raw hex

`.banner.error` uses background `#f4e4dc` (`forms.css`). It is a danger wash, and it is the only raw color outside `tokens.css` that a service should keep. The next time a second host needs it, add `--danger-wash: #f4e4dc` to `tokens.css` on the hub and replace the hex in both places. Until then, copy the rule as written.

---

## 5. Type

### 5.1 Faces

Both families are variable fonts, one file per subset, `font-display: swap`, weights 100–900.

| File | Face |
|---|---|
| `fraunces-latin.woff2` | Fraunces, normal |
| `fraunces-latin-ext.woff2` | Fraunces, latin-ext (this is where `₽` lives) |
| `public-sans-latin.woff2` | Public Sans, normal |
| `public-sans-latin-ext.woff2` | Public Sans, latin-ext |
| `public-sans-italic-latin.woff2` | Public Sans, italic |
| `public-sans-italic-latin-ext.woff2` | Public Sans, italic latin-ext |

`index.html` preloads the two latin files:

```html
<link rel="preload" as="font" type="font/woff2" href="/fonts/fraunces-latin.woff2" crossorigin />
<link rel="preload" as="font" type="font/woff2" href="/fonts/public-sans-latin.woff2" crossorigin />
```

Fallbacks live at the bottom of `base.css` (`Public Sans Fallback` at `size-adjust: 107%`, `Fraunces Fallback` at `104%`) so the swap does not move the page. Services that can render Cyrillic or `₽` will fetch latin-ext on their own; an English screen does not.

No request to `fonts.googleapis.com`.

### 5.2 Roles

From `base.css` and `article.css`:

| Role | Spec |
|---|---|
| Body | Sans, `--step-0`, line-height `1.65`, letter-spacing `0.01em`, antialiased |
| `h1`–`h3` | Serif, weight `650`, letter-spacing `-0.03em`, line-height `1.15`, margin `0` |
| Wordmark | Serif, `--step-1`, weight `650`, letter-spacing `-0.02em` |
| Kicker | `0.72rem`, weight `600`, letter-spacing `0.18em`, uppercase, `--accent-text`, margin `0 0 var(--s3)` |
| Meta line | `--ink-muted`, `--step--1`, letter-spacing `0.04em` |
| Lede | `--step-1`, line-height `1.5`, margin `0 0 var(--s5)` |
| Prose link | `--accent-text`, underline 1px, offset 2px |
| Nav link | No underline. `--ink-muted`, weight `600`, `--step--1`. Hover `--accent-text` |
| Big number | Serif, `--step-2`, tabular figures, `--accent-text` |

`.kicker` sits above the `h1` and names the place: `Sign in`, `Admin`, `Resume`, or the service's section. One `h1` per view. The `h1` is the page title in the type scale, which is `--step-4`.

### 5.3 Column

Reading text sits in `--measure` (`34rem`), centered in the page. A directory, a file list, or a canvas may run wider than that — the admin table does — and it still uses `.main` padding. The kicker and `h1` stay at the type scale even when the tool below them is wide.

---

## 6. Surfaces, shape, depth

| Surface | Treatment |
|---|---|
| Page | `--paper`. No gradient, no ruled lines. |
| Grain | One `body::before`: fixed, `inset: 0`, `pointer-events: none`, `z-index: 0`, opacity `0.025`, the fractal-noise SVG already in `base.css`. It does not repaint on scroll. `.shell` is `z-index: 1` so content sits above it. |
| Text | Directly on the paper. |
| Raised | `--paper-raised` plus `1px solid var(--rule)`. Used for `.panel`, `.a-figure`, `.table-wrap`. |
| Hairline list | Rows separated by `border-top: 1px solid var(--rule)`, last row also bordered. No fill. |
| Chips | `--chip` fill, `1px solid var(--rule)`, `--step--1`, padding `0.1rem 0.5rem`. |

Corners are square. The focus ring uses `border-radius: 1px` so the outline does not look bitten. Circles exist for the diamond's dot and, on the hub, the owl. Rounded rectangles of 5px, 10px, 14px, and 16px exist only in `chat.css`.

Shadows exist only on the owl bubble and the owl chat panel. A new service stylesheet has no `box-shadow`.

Selection is `--accent` with `--paper` text.

---

## 7. Controls

Copy `web/src/styles/forms.css`. The class names are the API.

### 7.1 Buttons

`.btn` is an inline-flex control, square, `min-height: 2.75rem`, padding `0.7rem 1.1rem`, weight `600`. Fill and border are `--ink`, label is `--paper`. Hover turns both fill and border to `--accent-text`. Transition is `background` over `--dur-fast`.

| Class | Look |
|---|---|
| `.btn` | Ink fill. The one primary action: Sign in, Continue with Google, an outbound action. |
| `.btn-ghost` | Transparent, ink label. Hover fills `--chip`. Sign out. |
| `.btn-danger` | Transparent, `--danger` label and border. Hover fills `--danger` with a `--paper` label. |
| `.btn-small` | `min-height: 2.25rem`, padding `--s2 --s3`, `--step--1`. Header controls. |

A button and a link that does the same job share `.btn`. Disabled controls are `opacity: 0.55` with `cursor: not-allowed`.

### 7.2 Fields

`input`, `select`, and `button` inherit the body font (`base.css`).

Text fields and selects: background `--paper`, border `1px solid var(--rule)`, color `--ink`, square. Search padding is `--s2 --s3`. Selects add `--s1 --s2` and `min-width: 7.5rem`.

The label above a field matches a table header: `0.72rem`, weight `600`, letter-spacing `0.12em`, uppercase, `--ink-muted`.

### 7.3 Banners, tables, empty

| Class | Look |
|---|---|
| `.banner` | `--chip` fill, `1px solid var(--rule)`, padding `--s3 --s4`, margin below `--s4` |
| `.banner.error` | `--danger` border and text, wash `#f4e4dc` |
| `.table-wrap` | Raised surface, horizontal scroll |
| `th` | Uppercase label style on `--chip` |
| `td` | `--s3` padding, hairline below, `white-space: nowrap`, last row has none |
| `.empty` | Centered, `--ink-muted`, padding `--s6 --s4` |
| `.status-blocked` | `--danger`, weight `600`. A `select` with the class also takes a danger border |

`.panel` is the auth card: raised, padding `--s6`, `max-width: 28rem`. `.stack` is a column with `--s4` gaps. A dialog or a short form uses `.panel`, which has no shadow and no radius. That is the floating surface a service is allowed to add.

### 7.4 Choosing rows

When the reader picks a next place — a folder, a thread, a document — use a hairline row, the way `.branch-card` does:

- The whole row is one link. Minimum height `4.5rem`. Padding `--s4 --s2 --s4 --s3`.
- Grid: a serif tabular index in `--accent-text` at `--step--1`, then the title at `--step-1` weight `600`, then a muted teaser at `--step--1` line-height `1.45`, then optional meta.
- A `2px` `--accent` bar on the left, `scaleY(0)` to `scaleY(1)` from the top over `--dur-fast`, on hover and on `:focus-visible`.
- The title turns `--accent-text` on that same hover.

At `760px` the meta wraps under the title. The row stays one link.

---

## 8. Chrome

Every host renders the same shell. Copy `shell.css` through `.footer` and both media queries. Stop before the comment `the owl`.

```
┌──────────────────────────────────────────────────────────────┐
│  ◆ Filenkov     Messanger · Storage · …          [ Sign in ] │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│   KICKER                                                     │
│   Page title                                                 │
│   One muted sentence about the page.                         │
│                                                              │
│   (the service)                                              │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│  Filenkov · Storage          Google sign-in · httpOnly …    │
└──────────────────────────────────────────────────────────────┘
```

### 8.1 Landmarks

Order inside `.shell`:

1. Skip link, first focusable element. Label "Skip to content". Target is the `<main>` id. Visible only on `:focus-visible`: ink fill, paper text, weight `600`, translated in from above.
2. `<header class="header">`.
3. `<main class="main" id="main">`.
4. `<footer class="footer">`.

`.shell` is a column, `min-height: 100svh`, `padding-top: var(--header-h)`. `.main` is `flex: 1`, padding `--s5 --s5 --s8`. The footer is a hairline (`border-top: 1px solid var(--rule)`), `--ink-muted`, `--step--1`, space-between, wrapping. Left side: `Filenkov · <service name>`. Right side: `Google sign-in · httpOnly cookies on .filenkov.store`.

### 8.2 Brand

The header mark is a 26px diamond with a copper dot. The favicon (`web/public/favicon.svg`) is the owl; that stays the tab icon on every host. The header does not use the owl.

```svg
<svg class="brand-mark" viewBox="0 0 32 32" aria-hidden="true">
  <path d="M16 3 L29 16 L16 29 L3 16 Z" fill="none" stroke="currentColor" stroke-width="1.75" />
  <circle cx="16" cy="16" r="3.2" fill="var(--accent)" />
</svg>
```

The wordmark is the text `Filenkov`. The link goes to `https://filenkov.store` (on the hub itself it is an in-app link to `/`). Color `--ink`, no underline, gap `--s3` between mark and word.

### 8.3 Service links

`<nav class="service-nav" aria-label="Primary">`. Same set, same order, same show/hide rules as `Layout.tsx`:

| Link | Shown | Guest |
|---|---|---|
| Messanger → `https://messanger.filenkov.store` | Always | Visible, with a lock, `title="Sign in required"` |
| Storage → `https://storage.filenkov.store` | Always | Same lock |
| Hermes → `https://filenkov.store/hermes/chat` | Always | Same lock |
| Image → `https://image.filenkov.store` | Only when the `image` service role is `FAMILY` or `ADMIN` | Hidden. Never locked |
| Chat → `https://llm.filenkov.store` | Only when hub `global_role` is `ADMIN` | Hidden. Never locked |
| Admin → `https://filenkov.store/admin` | Only when hub `global_role` is `ADMIN` | Hidden |

The lock is the character `🔒` in `.service-lock` (`0.7em`, `margin-left: 0.25em`, `aria-hidden`). Guests can click Messanger, Storage, and Hermes; the gateway sends them through sign-in and back. Image, Chat, and Admin stay hidden unless the role check passes, because a link that 403s is noise.

The hub reads those roles from `GET /api/me` (`global_role` and `services[]`). A product host only receives `X-Auth-Role`, which is the hub `global_role`, so Chat and Admin can follow that header. The `image` role is not in it. A product header shows Image when its own backend can supply that role, and omits Image otherwise.

On a service host, that host's own link is the current item: color `--ink`, weight `650`, `aria-current="page"`, and it is not a link to itself.

### 8.4 Sign-in slot

Right side, `margin-left: auto`.

| State | What the header shows |
|---|---|
| Session unknown | `<span class="muted">Checking sign-in…</span>` |
| Guest | `<a class="btn btn-small" href="https://filenkov.store/auth?return_to=<absolute current URL>">Sign in</a>` |
| Signed in | `<span class="user-name">{username}</span>` and a ghost `.btn-small` labelled `Sign out` |

The name in the header is `username`. Email stays in directories and account detail.

Where that name comes from, and what Sign out posts to, is §9. The hub plays an owl departure before logout. A service disables the button while the request is in flight and skips the owl.

### 8.5 Header motion

The header is `position: fixed`, `z-index: 20`, height `--header-h`, horizontal padding `--s5`, gap `--s5`. `html[data-header]` has three values, written by a scroll listener that never calls `setState`:

| `data-header` | When | Look |
|---|---|---|
| `top` | Scroll under 8px | Full height, transparent, wordmark and service links visible |
| `hidden` | Scrolling down, past 96px, and the header does not contain focus | `translateY(-100%)` |
| `condensed` | Scrolling up, or focus is inside the header | Height `--header-h-c`, background `color-mix(in srgb, var(--paper) 82%, transparent)`, `backdrop-filter: blur(8px)`, `1px` bottom rule. Wordmark and service nav fade (`opacity: 0`, `pointer-events: none`). Mark and sign-in stay |

Constants from `useScrollChrome.ts`, copied with the header:

| Name | Value | Meaning |
|---|---|---|
| `JITTER_FLOOR` | 6px | Smaller frame-to-frame deltas are trackpad noise |
| `TRAVEL_THRESHOLD` | 24px | Accumulated travel in one direction before the state flips |
| `TOP_AT` | 8px | Below this the header is always `top` |
| `HIDE_BELOW` | 96px | The header does not hide above this |

`last` advances only after a frame clears the jitter floor, so a slow drift still adds up. A keyboard user tabbing inside the header forces `condensed`; the bar is translated off-screen and remains focusable, so `display: none` and `inert` are the wrong tools. Transition is `transform` and `height` over `--dur` / `--ease`, which is what `shell.css` ships. Wordmark and nav fade over `--dur-fast`.

A page that does not scroll can leave `data-header` at `top`.

### 8.6 Breakpoints

| Width | Change |
|---|---|
| `760px` | `.main` and header padding shrink to `--s4`. Header gap becomes `--s4`. A choosing-row's meta wraps under the title. |
| `620px` | The wordmark hides. The mark still names the product. Service links keep `--s3` gaps and scroll on X with the scrollbar hidden. |

`1080px` is the hub's spine breakpoint. A service with a side column uses it the same way: drop the side column before stacking the page.

---

## 9. Sign-in, from the service's side

The Google button lives on `filenkov.store/auth`. A service paints no second one.

Cookies are `auth_access` and `auth_refresh`, httpOnly, `SameSite=Lax`, on `.filenkov.store`. The Auth API is same-origin behind the gateway and has no CORS (`README.md`). `GET /api/me`, `POST /api/refresh`, and `POST /api/logout` are routed on `filenkov.store` only (`deploy/gateway.yaml`). A page on `storage.filenkov.store` or any other product host cannot `fetch` them.

What a product host can do today:

- **Sign in** is a top-level navigation to `https://filenkov.store/auth?return_to=<current absolute URL>`. `return_to` must be `https` on a host listed in `deploy/gateway.yaml`. `continueTarget` in `web/src/pages/Auth.tsx` is that allowlist. The gateway already refreshes an expired access cookie before it proxies, so the product UI never calls `/api/refresh` itself.
- **Who is signed in** arrives as gateway headers on the product's own backend: `X-Auth-Name` (username), `X-Auth-Email`, `X-Auth-User-Id`, `X-Auth-Role` (hub `global_role`). The product's same-origin API returns the username to its page. The header slot in §8.4 renders that username.
- **Sign out** has no product-host route yet. `POST /api/logout` exists on the hub, and a cross-origin `fetch` will not clear the cookies. Until a host grows a same-origin logout that proxies to the Auth API (still no CORS on the Auth API), Sign out navigates to `https://filenkov.store`. The hub header there is the button that posts `/api/logout`. Opening the hub does not sign the reader out on its own. A same-origin logout is a gateway change for that host's user story. It is not a new color and not a second session.

Roles rendered anywhere use the exact strings `ADMIN`, `FAMILY`, `STRANGER`, `BLOCKED`. `BLOCKED` is `.status-blocked`.

Hub `global_role` and a service role are different facts. Image is shown from the `image` role. Chat and Admin are shown from `global_role`. A service paints its own authorization from the role the gateway already enforced, and it keeps the same four words.

---

## 10. Page anatomy

### 10.1 Head

Every view opens the same way the admin page does:

```html
<p class="kicker">Storage</p>
<h1>Files</h1>
<p class="muted">One sentence. What this screen is for.</p>
```

The kicker is the service or the section. The `h1` is the view. The muted line is optional and uses the admin lead's spacing when it sits tight under a title: a small negative top margin is already `.admin-lead` (`margin: calc(var(--s2) * -1) 0 var(--s5)`). A service may reuse that class.

### 10.2 Prose

When a screen explains something, compose it from `web/src/components/article/index.tsx` and plain `p` / `ul` / `ol`:

| Piece | Class | What it is |
|---|---|---|
| `Section` | `.a-section` | The only `h2`. Top rule, margin `--s7`, padding-top `--s5`. Its `h2` is `--step-2`. |
| `Chips` | `.a-chips` | A wrap of tags. Empty input renders nothing. |
| `Figure` | `.a-figure` | The raised surface. Image is `max-width: 100%`, lazy, async. `scroll` lets a wide figure scroll inside, with `min-width: 34rem`. Caption is muted `--step--1`. |
| `Callout` | `.a-callout` | `2px` left `--accent` rule, padding-left `--s4`. Optional title at weight `650`. |
| `Metrics` / `Metric` | `.a-metrics`, `.a-metric` | Hairline rows. Value in serif copper, then label, then a muted "how" line. The third line is the point of the component. |
| `Timeline` | `.a-timeline` | Left hairline. When-line is uppercase `--accent-text` at `--step--1`, tracking `0.08em`. |
| `DefinitionList` | `.a-deflist` | Term at weight `650`, detail muted, hairline under each pair. |

Body copy inside `.node-body` (or the same rules, renamed) has paragraphs at margin `0 0 var(--s4)`, lists padded `--s5`, and `--s2` between items.

### 10.3 States

| State | UI |
|---|---|
| Session loading | The sentence `Checking sign-in…` in `.muted`. Nothing else shifts. |
| List loading | `.empty` with a specific sentence (`Loading directory…`). |
| Fast load | `useDelayedFlag` waits 120ms before a skeleton is allowed to paint. A response that is already here never flashes a skeleton. |
| Skeleton | Bars of `--rule` at opacity `0.55`, height `0.75rem`, gap `--s3`. |
| Empty | `.empty` with a specific sentence (`No one matches that email.`). |
| Failed action | `.banner.error` with the error text. The rest of the page stays. |
| Failed view | A `2px` left `--danger` rule and `--danger` text, as `.node-error`. Offer one recovery control. |
| Destructive role | `.status-blocked` on the text or the select. |

Loading copy names the thing that is loading. A spinner is not part of the language.

### 10.4 Document head

`lang="en"`. `color-scheme: light`. Viewport `width=device-width, initial-scale=1`. One title of the form `<view> · <service> · Filenkov`. A description meta tag. The favicon is `/favicon.svg`.

---

## 11. Motion, focus, targets

| Thing | Spec |
|---|---|
| Entering a view | Opacity `0→1` and `translateY(8px→0)` over `--dur`. Never translate on X. |
| Header | §8.5 |
| Row hover | Left accent bar `scaleY`, `--dur-fast` |
| Button hover | Background color, `--dur-fast` |
| Anything else | Still |

Focus, from `base.css`, on every host:

```css
:focus-visible {
  outline: 2px solid var(--accent-text);
  outline-offset: 2px;
  border-radius: 1px;
}
```

Hit targets are at least 44×44px for header buttons, rows, and anything that is the primary tap on a phone. `.btn` at `2.75rem` meets that. `.btn-small` is the header exception the hub already ships (`2.25rem`); keep it in the header, and keep primary actions on `.btn`.

A route change that replaces the `h1` moves focus to that heading (`tabIndex={-1}`) and writes the title into a visually hidden `aria-live="polite"` region. The first load of a document does not steal focus.

`.sr-only` is the clip pattern at the bottom of `base.css`.

---

## 12. Files to lift

Copy verbatim:

| Path | Notes |
|---|---|
| `web/src/styles/tokens.css` | The whole contract |
| `web/src/styles/fonts.css` | Subsets and unicode-range |
| `web/src/styles/base.css` | Reset, grain, type, focus, fallbacks |
| `web/src/styles/forms.css` | Controls. Admin class names included; they are the form language |
| `web/public/fonts/*.woff2` | Six files |
| `web/public/favicon.svg` | Tab icon |
| `web/src/hooks/useDelayedFlag.ts` | 120ms skeleton gate |

Copy and stop at the owl:

| Path | Keep | Drop |
|---|---|---|
| `web/src/styles/shell.css` | `.shell` through the `620px` media query | From `/* --- the owl` to the end of `.owl-ring` |
| `web/src/hooks/useScrollChrome.ts` | The `data-header` machine and its constants | `data-owl`, `--read`, `setOwlState` |
| `web/src/components/Layout.tsx` | Mark, service list, sign-in slot, skip link, footer | `Owl` and the departure animation |

Copy when the screen has prose or a directory of choices:

| Path | Keep |
|---|---|
| `web/src/styles/article.css` | All of it |
| `web/src/components/article/index.tsx` | All of it |
| `.branch-card` rules in `web/src/styles/tree.css` | The row, the bar, the index, the teaser. Rename them if `branch` would lie |

Import order, for the files a service actually uses:

```ts
import './styles/fonts.css'
import './styles/tokens.css'
import './styles/base.css'
import './styles/shell.css'
import './styles/article.css' // omit on a tool with no prose
import './styles/forms.css'
```

Leave `tree.css` and `chat.css` on the hub.

---

## 13. Checklist before a service ships

A host looks like the others when every line below is true.

1. The page background is `--paper`, the grain is the one from `base.css`, and the stylesheet contains no `box-shadow` and no new hex.
2. Copper text is `--accent-text`. `--accent` is only a mark, a bar, a selection, or a stroke.
3. The header shows the diamond, the wordmark linking home, the service list from §8.3 with the current host marked, and either `Checking sign-in…`, `Sign in`, or `username` plus `Sign out`.
4. Guests reach the hub `/auth` with `return_to`. The service draws no Google button.
5. The first focusable control is the skip link. The view has one `h1` under a kicker. Buttons, fields, banners, and tables use the classes in `forms.css`.
6. A list the reader chooses from is hairline rows with the accent bar. A short form sits in `.panel`.
7. Loading, empty, and error copy are specific sentences. Skeletons wait 120ms.
8. At 620px the wordmark is gone and the service links still scroll. At 760px the page padding is `--s4`.
9. `prefers-reduced-motion` collapses the two durations, and a keyboard walk shows the focus ring on every stop.
10. `lang` is `en`, `color-scheme` is `light`, fonts are the self-hosted files, and the tab icon is the shared favicon.

---

## 14. Keeping one language

Change the hub's `tokens.css` first, then port that diff. A service that needs a color the table in §4 does not have adds the token on the hub in the same change, with a one-line note of what it is for.

A shared package is the right move once two services outside this repo are copying these files and drifting. Until that second consumer exists, the copy in §12 is the contract, and this document is the checklist that keeps the copies honest.
