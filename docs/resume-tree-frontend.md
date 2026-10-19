# Resume tree frontend — design document

Status: **shipped**. What was built, and where it diverges from this document, is recorded in §16.
Scope: `web/` only. No Go changes, no `deploy/gateway.yaml` changes (see §6.4).
Audience: whoever writes the code (you, or an agent working from this file).

---

## 1. What we are building and why

The hub page at `filenkov.store` currently prints the whole resume on one scroll — visit card, Stack, Projects, Experience, Architecture — with `RevealTree` accordions that expand in place. Every reader gets every word in the same order, and the page only grows.

We replace that with a **tree you walk**. The reader lands on the root (the visit card), sees a small number of branches, and picks the one they care about. Each step opens one article-length node with its own URL, loaded on demand. A recruiter goes `Start → For interviewer` in one click. An engineer goes `Start → Experience → Sber → 2× cheaper tokens` and never sees the logistics. Nobody scrolls past what they did not ask for.

Four rules govern everything below.

| # | Rule | Where it is enforced |
|---|---|---|
| 1 | Minimalism, the feel of a paper article | §3 design language |
| 2 | The resume is a tree; root at top, reader chooses the next stage; scrollable column in the middle, clickable nodes | §4 layout, §5 tree model |
| 3 | Nodes must be trivial to add, edit, reorder, replace | §5.2 directory-as-path registry |
| 4 | A node's content loads only when the reader opens it | §5.4 code splitting + §5.5 prefetch |

Plus the chrome: §7 auto-hiding header with service links and sign-in, §8 the owl that becomes a back-to-top button.

### 1.1 Decisions already taken

* Layout: **spine + branch cards** (not a drawn mini-map, not a pan/zoom canvas).
* Node authoring: **one `.tsx` module per node**, lazily imported.
* `Stack` moves under **For interviewer**. `Architecture` moves under **About this project**. `Projects` moves under **Achievements**.
* Language: **English only.** No i18n layer, no language dimension in the registry. (If Russian is ever added, §5.2 gains a `lang` suffix on the module filename and nothing else changes.)

---

## 2. Current state — what we keep, change, delete

| File | Fate |
|---|---|
| `src/main.tsx` | keep |
| `src/App.tsx` | **rewrite** — routes become the tree (§6) |
| `src/auth.tsx` | keep as-is |
| `src/api.ts`, `src/types.ts` | keep as-is |
| `src/components/Layout.tsx` | **rewrite** — header state machine, owl leaves the header (§7) |
| `src/components/HeaderMascot.tsx` | **becomes** `src/components/owl/Owl.tsx` (§8); the mood-clip logic survives intact |
| `src/components/RevealTree.tsx` + test | **delete** — real navigation replaces the accordion |
| `src/pages/Home.tsx` + test | **becomes** the root node `src/resume/nodes/node.tsx` |
| `src/pages/Architecture.tsx` + test | **becomes** `src/resume/nodes/about-project/architecture/[slug]` (§9.4) |
| `src/pages/Auth.tsx`, `src/pages/Admin.tsx` + tests | keep, unchanged, still at `/auth` and `/admin` |
| `src/resume/data.ts` | **dissolved** into node modules (§9); the file is deleted at the end of Phase 2 |
| `src/index.css` (807 lines, one file) | **split** into `src/styles/*.css` (§3.6) |
| `src/assets/animations/*.svg` | keep — these are the owl morphs, still used |
| `src/assets/architecture.svg` (128 KB) | keep, but move to `nodes/about-project/architecture/` and lazy-load (§10) |
| `index.html` | change: self-hosted fonts instead of Google Fonts (§10.3) |
| `web/nginx.conf` | **no change** — `try_files … /index.html` already makes deep links work |
| `deploy/gateway.yaml` | **no change** — the trailing `- match: /*  upstream: web  auth: public` already serves every new node path publicly |

Nothing in `cmd/`, `internal/`, or the Go tests is touched.

---

## 3. Design language — "paper article"

### 3.1 Principle

A printed article has: one column of text at a comfortable measure, generous margins, hairline rules instead of boxes, one accent ink, and no drop shadows. Every deviation from that must earn its place. When in doubt, **remove the box and draw a rule**.

The existing palette is already right. What is wrong today: shadows (`--shadow` on cards), a ruled-paper background that fights the text baseline, cards with backgrounds inside a page that is itself a card, and no focus ring.

### 3.2 Tokens (`src/styles/tokens.css`)

```css
:root {
  /* ink */
  --paper:        #f4efe6;   /* page */
  --paper-raised: #fffdf8;   /* the one raised surface we allow: figures */
  --ink:          #1a1612;   /* body text */
  --ink-muted:    #5c5348;   /* secondary text — 6.4:1 on paper, AA ✓ */
  --rule:         #d9d1c3;   /* hairlines */
  --rule-strong:  #b9ae9c;   /* the spine, active rules */

  /* accent — TWO tokens, not one */
  --accent:       #b85c38;   /* graphical only: dots, bars, 3px+ marks */
  --accent-text:  #8f4128;   /* any accent-coloured TEXT — 6.3:1 on paper, AA ✓ */

  --danger:       #8c2f24;

  /* type */
  --serif: 'Fraunces', 'Iowan Old Style', 'Palatino Linotype', serif;
  --sans:  'Public Sans', system-ui, 'Segoe UI', sans-serif;

  --step--1: 0.8125rem;   /* meta, chips, captions */
  --step-0:  1.0625rem;   /* body */
  --step-1:  1.25rem;     /* lede */
  --step-2:  1.5rem;      /* h3 */
  --step-3:  1.9rem;      /* h2 */
  --step-4:  clamp(2.2rem, 4.5vw, 3.1rem);  /* h1 */

  --measure: 34rem;       /* ≈66 characters at --step-0 */

  /* space — 4px base, no arbitrary values anywhere else */
  --s1: 0.25rem; --s2: 0.5rem;  --s3: 0.75rem; --s4: 1rem;
  --s5: 1.5rem;  --s6: 2rem;    --s7: 3rem;    --s8: 4.5rem;

  /* motion — exactly two durations and one curve */
  --dur-fast: 140ms;
  --dur:      240ms;
  --ease:     cubic-bezier(0.2, 0, 0, 1);

  --header-h:   4.5rem;   /* expanded */
  --header-h-c: 3.5rem;   /* condensed */
}
```

**Contrast fix worth calling out.** Bright copper `#b85c38` on paper is ≈4.0:1 — it fails AA for body-size text, and today it is used for `.kicker`, `.reveal-meta` links, `.resume-contact-label`, `.arch-card-more`. Split into two tokens: `--accent-text` for anything a person reads, `--accent` only for dots, bars, rules and icons ≥3px. Mechanical find-and-replace; visually near-identical.

### 3.3 Background

Delete the two-layer `radial-gradient` + `repeating-linear-gradient` currently on `body`. The 24px rule lines beat against the 27px body line-height and produce a faint moiré on scroll — it is the single biggest reason the current page does not feel "smooth". Replace with flat `--paper` plus, optionally, one static SVG grain at `opacity: .025` as a `background-image` on `body::before` (`position: fixed; inset: 0; pointer-events: none;` so it never repaints during scroll).

### 3.4 Surfaces

* The page is paper. It has no card.
* Article text sits directly on the page. No panel, no border, no shadow.
* The **only** raised surface is a figure (the architecture SVG, a screenshot): `--paper-raised` + 1px `--rule`, no shadow.
* Branch cards are **rows separated by hairlines**, not boxes (§4.4).
* `--shadow` is deleted from the token set. If a shadow appears in a diff, it is a bug.

### 3.5 Motion

| Thing | Spec |
|---|---|
| Node change | outgoing `opacity → 0` over `--dur-fast`; incoming `opacity 0→1` + `translateY(8px→0)` over `--dur`. Never translate on X — it fights the scrollbar. |
| Header show/hide | `transform: translateY()` only, `--dur`, `--ease`. Never animate `height` or `top`. |
| Owl hero → button | `transform` only, `--dur` (§8.3) |
| Branch card hover | left accent bar `scaleY(0→1)` from the top, `--dur-fast` |
| Everything else | no animation |

`@media (prefers-reduced-motion: reduce)`: every duration above collapses to `1ms`, the owl morph SVGs are replaced by their static resting frame (`My_icon_calm.svg`), and `scrollTo` uses `behavior: 'auto'`.

### 3.6 Stylesheet split

`index.css` becomes:

```
src/styles/
  tokens.css      the :root block above
  base.css        reset, body, links, :focus-visible, typography defaults
  shell.css       .shell, header, footer
  article.css     the node column: headings, lede, p, ul, figure, chips
  tree.css        .spine, .branch-cards, .node-skeleton
  forms.css       .btn, select, input, table  (Auth + Admin pages)
```

`main.tsx` imports them in that order. Each file stays under ~150 lines, so a change to the spine never risks the admin table.

**Focus ring** (missing today, add to `base.css`):

```css
:focus-visible {
  outline: 2px solid var(--accent-text);
  outline-offset: 2px;
  border-radius: 1px;
}
```

---

## 4. Screen anatomy

### 4.1 At the top of the page (scroll = 0)

```
┌──────────────────────────────────────────────────────────────┐
│  ◆ Filenkov        LLM · Storage · Hermes 🔒      [ Sign in ] │  header, expanded
├──────────────────────────────────────────────────────────────┤
│                                                              │
│                          ( owl )                             │  hero owl, in flow
│                                                              │
│   ┌ spine ┐   ┌───────── article column ─────────┐           │
│   │       │   │  RESUME                          │           │
│   │  ● Start  │  Alexey Filenkov                 │           │
│   │       │   │  Senior AI Agents                │           │
│   │       │   │  I build AI agents for …         │           │
│   │       │   │                                  │           │
│   │       │   │  ─── Where next? ───────────────  │           │
│   │       │   │  01  Experience            →     │           │
│   │       │   │  02  Achievements          →     │           │
│   │       │   │  03  For interviewer       →     │           │
│   │       │   │  04  About this project    →     │           │
│   └───────┘   └──────────────────────────────────┘           │
└──────────────────────────────────────────────────────────────┘
```

### 4.2 Two levels deep, scrolled down

```
┌──────────────────────────────────────────────────────────────┐
│  ◆                                              [ Sign in ]  │  header, condensed
├──────────────────────────────────────────────────────────────┤
│   ┌ spine ┐   ┌───────── article column ─────────┐           │
│   │  ○ Start  │  Sber                            │           │
│   │  │        │  2023 — 2026 · 3 years           │           │
│   │  ○ Exper. │                                  │           │
│   │  │        │  Automated reporting: ~1M RUB…   │           │
│   │  ● Sber   │  … body scrolls …                │           │
│   │  ╵        │                                  │      ╭──╮ │
│   └───────┘   │  ─── Deeper ───                  │      │🦉│ │  owl → button
│               │  01  2× cheaper tokens     →     │      ╰──╯ │
│               │  02  Reports that remember →     │           │
│               │  03  Tools as MCP          →     │           │
│               │  ─── Or sideways ───             │           │
│               │  AeroClub · Education            │           │
│               │  ← Up to Experience              │           │
│               └──────────────────────────────────┘           │
└──────────────────────────────────────────────────────────────┘
```

### 4.3 Grid

```css
.node-layout {
  display: grid;
  grid-template-columns: minmax(0, 13rem) minmax(0, var(--measure)) minmax(0, 13rem);
  justify-content: center;
  gap: var(--s6);
}
```

Left column = spine (sticky). Centre = the scrolling article. Right column is **empty on purpose** — it is the outer margin of the sheet, and it keeps the article optically centred instead of shoved left. The owl button floats over it when docked.

Breakpoints:

* `> 1080px` — three columns as above.
* `760–1080px` — two columns, spine `10rem`, no right column, article `--measure` shrinks naturally.
* `< 760px` — one column. The spine becomes a **horizontal breadcrumb strip** sticking directly under the header: `Start › Experience › Sber`, `overflow-x: auto`, `scroll-snap-align: end` on the last chip so the current node is always in view. Height `2.75rem`, hairline bottom.

### 4.4 The spine

Not a diagram — a vertical hairline with a dot per node on the path from root to the current node.

* `<nav aria-label="Resume path"><ol>` — one `<li>` per ancestor plus the current node.
* Ancestors: `○` outline dot, `--ink-muted` label, clickable `<Link>` back to that node.
* Current: `●` filled `--accent` dot, `--ink` label, `aria-current="page"`, not a link.
* A 1px `--rule-strong` line runs behind the dots. Below the current dot it fades out (`mask-image: linear-gradient(#000, transparent)`) to say "the path continues if you choose".
* `position: sticky; top: calc(var(--header-h-c) + var(--s5));`
* Depth never exceeds 4, so the spine never scrolls internally.

### 4.5 Branch cards ("Where next?")

Rows, not boxes. Each row:

```
┌ 2px accent bar (scaleY 0 → 1 on hover/focus)
│  01   Experience                                         →
│       Five years, two companies, numbers attached.
└─────────────────────────────────────────────── hairline ──
```

* `<ul>` of `<li><Link>` — real links, so middle-click and "open in new tab" work.
* Number is `--serif`, `font-variant-numeric: tabular-nums`, `--accent-text`.
* Title `--step-1`, teaser `--step--1` in `--ink-muted` — **the teaser comes from `meta.ts`, which is already loaded**, so it costs nothing.
* Min height `4.5rem` → hit target well over 44px.
* Hover/focus: title → `--accent-text`, left bar grows. No lift, no shadow, no background change.

Three groups appear under every node, each only if non-empty:

1. **Deeper** — the node's children.
2. **Or sideways** — the node's siblings, rendered as a compact inline list of links, not cards.
3. **← Up to `<parent>`** — a single ghost link. On the root, this group is absent.

Siblings matter: a reader inside `Sber` must be able to reach `AeroClub` without going back up.

---

## 5. The tree — data model and file structure

### 5.1 The shape

```
/                                Start — the visit card
├── /experience                  Experience
│   ├── /experience/sber                 Sber · 2023–2026
│   │   ├── …/cheaper-tokens                     2× cheaper tokens
│   │   ├── …/report-memory                      Reports that remember
│   │   └── …/mcp-tools                          Tools as MCP
│   ├── /experience/aeroclub             AeroClub · 2021–2023
│   │   ├── …/mail-routing                       Multi-label mail routing
│   │   ├── …/ner                                NER at 80%+
│   │   └── …/human-in-the-loop                  HITL framework
│   └── /experience/education            MAI, MPEI
├── /achievements                Achievements
│   ├── /achievements/numbers            The numbers, with how they were measured
│   └── /achievements/projects           Projects  ← moved here
│       ├── …/auth-hub                           filenkov.store
│       ├── …/storage                            storage.filenkov.store
│       ├── …/llm                                llm.filenkov.store
│       ├── …/resume-studio                      vacancy → tailored PDF
│       └── …/hermes                             signed-in only
├── /for-interviewer             For interviewer
│   ├── /for-interviewer/stack           Stack  ← moved here
│   │   ├── …/ai-agents                          LangGraph, RAG, MCP
│   │   ├── …/backend                            Python, FastAPI, Go, Postgres
│   │   └── …/infra                              Caddy, Docker, MinIO
│   ├── /for-interviewer/how-i-work      Process, review, what I need from a team
│   └── /for-interviewer/logistics       Availability, English B1, location, contacts
└── /about-project               About this project
    ├── /about-project/why-a-tree        The design argument
    ├── /about-project/architecture      Architecture  ← moved here (the SVG map)
    │   └── /about-project/architecture/:slug   10 solution pages (§9.4)
    └── /about-project/site-stack        React 19, Vite, Go gateway, how it deploys
```

Counting the root as depth 0, the deepest content node is depth 3 (`experience/sber/cheaper-tokens`), and architecture slug pages are also depth 3. The registry test caps depth at **4**, leaving one level of headroom (§12). Beyond that the spine stops being readable and the reader stops knowing where they are.

### 5.2 Directory is the path — the whole trick

One rule: **a node is a folder under `src/resume/nodes/`, and its folder path is its URL path.**

```
src/resume/
  types.ts
  registry.ts
  registry.test.ts
  redirects.ts
  nodes/
    meta.ts                       ← the ROOT node's metadata      → path ""      → URL /
    node.tsx                      ← the ROOT node's body
    experience/
      meta.ts                                                     → "experience" → /experience
      node.tsx
      sber/
        meta.ts                                                   → /experience/sber
        node.tsx
        cheaper-tokens/
          meta.ts
          node.tsx
      aeroclub/ …
    achievements/ …
    for-interviewer/ …
    about-project/ …
```

There is **no central list of nodes to keep in sync**. Vite's `import.meta.glob` discovers them:

```ts
// src/resume/registry.ts
const metaModules = import.meta.glob<{ default: NodeMeta }>(
  './nodes/**/meta.ts',
  { eager: true },                 // tiny; always in the main bundle
)
const bodyModules = import.meta.glob<{ default: ComponentType<NodeProps> }>(
  './nodes/**/node.tsx',           // lazy: one chunk per node
)

const toPath = (key: string) =>
  key.replace(/^\.\/nodes\//, '').replace(/\/?(meta\.ts|node\.tsx)$/, '')
//  './nodes/meta.ts'                  → ''
//  './nodes/experience/sber/meta.ts'  → 'experience/sber'
```

Consequences, which is the point of rule 3:

| Task | What you do |
|---|---|
| Add a node | create the folder, write `meta.ts` + `node.tsx`. It appears in its parent's branch cards and at its URL. Nothing else. |
| Reorder siblings | change `order` in `meta.ts` |
| Rename / move a node | move the folder; add one line to `redirects.ts` if the old URL was public |
| Replace a node's content | rewrite `node.tsx`; `meta.ts` untouched |
| Retire a node | delete the folder; add a redirect |
| Hide a node from guests | `visibleWhen: 'signed-in'` in `meta.ts` |

Convention-over-configuration has one cost: a typo produces a missing node rather than a compile error. §12's `registry.test.ts` buys that back — it fails the build on any orphan, bad slug, reserved slug, duplicate order or over-deep path.

### 5.3 Types

```ts
// src/resume/types.ts
export type NodeVisibility = 'always' | 'signed-in' | 'admin'

export type NodeMeta = {
  /** Branch-card and spine label. Keep under ~28 chars. */
  title: string
  /** One line under the title on the branch card. Loaded eagerly — keep it short. */
  teaser: string
  /** Small right-aligned label: "2023 — 2026 · 3 years", "filenkov.store". */
  meta?: string
  /** Sibling sort key. Sparse (10, 20, 30) so inserting never renumbers. */
  order: number
  /** Overrides <title> and og:description; falls back to title/teaser. */
  seo?: { title?: string; description?: string }
  visibleWhen?: NodeVisibility      // default 'always'
  /** Renders a "leaf" ornament instead of a "branch" one when it has no children. */
  outbound?: { label: string; href: string }
}

export type ResumeNode = {
  path: string                 // '' for root, else 'experience/sber'
  segments: string[]
  depth: number                // root = 0
  parentPath: string | null
  childPaths: string[]         // already sorted by order, then title
  meta: NodeMeta
  load: () => Promise<{ default: ComponentType }>
}
```

### 5.4 Lazy loading — rule 4

`node.tsx` modules are **never** eagerly imported. `React.lazy(node.load)` inside a `<Suspense>` gives one Rollup chunk per node automatically.

```ts
// component cache so re-visiting a node does not re-create the lazy wrapper
const cache = new Map<string, LazyExoticComponent<ComponentType>>()
export function nodeComponent(node: ResumeNode) {
  let c = cache.get(node.path)
  if (!c) { c = lazy(node.load); cache.set(node.path, c) }
  return c
}
```

Chunk naming, so the network tab is readable:

```ts
// vite.config.ts
build: {
  rollupOptions: {
    output: { chunkFileNames: 'assets/[name]-[hash].js' },
  },
},
```

What stays in the **main** bundle: React, router, `auth.tsx`, `Layout`, `Owl`, `Spine`, `BranchCards`, and **every `meta.ts`**. Metadata for ~30 nodes at ~120 bytes each is ~4 KB — cheap, and it is what makes the spine, the branch cards and the skeleton header paint instantly.

### 5.5 Prefetch — where the "smooth" comes from

Lazy loading alone means a click is followed by a network wait. Prefetch removes the wait without loading everything up front.

```ts
// src/hooks/useNodePrefetch.ts
const warmed = new Set<string>()
export function prefetch(node: ResumeNode) {
  if (warmed.has(node.path)) return
  warmed.add(node.path)
  void node.load()                 // fires the dynamic import, result discarded
}
```

Triggers, in order of aggressiveness:

1. **`pointerenter` / `focus` on a branch card** — the strongest signal; fires ~200–400 ms before the click lands, which is usually the whole chunk.
2. **`pointerdown`** — belt and braces on touch, where there is no hover.
3. **Idle prefetch of direct children** after the current node has rendered:

```ts
useEffect(() => {
  const conn = (navigator as any).connection
  if (conn?.saveData || /2g/.test(conn?.effectiveType ?? '')) return
  const id = requestIdleCallback(() => current.childPaths.forEach(p => prefetch(byPath(p))))
  return () => cancelIdleCallback(id)
}, [current.path])
```

Only **direct** children — never the whole subtree. On a 4-branch root that is 4 chunks of ~6 KB.

### 5.6 Loading and error states

Because `meta` is already loaded, a "loading" node is not a spinner:

```
RESUME                       ← kicker, real
Sber                         ← real title, from meta
2023 — 2026 · 3 years        ← real meta line
▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓     ← skeleton lines, only the body
▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓
```

* The skeleton is **delayed by 120 ms** (`useDelayedFlag`). A prefetched chunk resolves in ~0 ms, so in the common case no skeleton is ever painted and there is no flash.
* The skeleton block reserves a min-height of `18rem` so the "Where next?" cards do not jump upward when the body arrives → **CLS 0**.
* Chunk load failure (the classic stale-deploy case, when a user has an old `index.html` and the hashed chunk is gone) is caught by an error boundary around `<Suspense>`: *"This section could not be loaded — the site was updated. [Reload]"*, where Reload calls `location.reload()`.

---

## 6. Routing

### 6.1 Routes

```tsx
// src/App.tsx
<BrowserRouter>
  <AuthProvider>
    <Routes>
      <Route element={<Layout />}>
        <Route path="/auth"  element={<Auth />} />
        <Route path="/admin" element={<Admin />} />
        <Route path="/architecture/:slug" element={<LegacyArchitectureRedirect />} />
        <Route path="*" element={<NodeRoute />} />
      </Route>
    </Routes>
  </AuthProvider>
</BrowserRouter>
```

`NodeRoute` takes `useParams()['*']`, normalises it (strip leading/trailing `/`, lowercase), and looks it up in the registry. Unknown path → `<Navigate to="/" replace />`. A node whose `visibleWhen` the current user fails → same.

### 6.2 Reserved slugs

Because nodes live at bare paths, no node segment may collide with a gateway route or an SPA page:

```ts
export const RESERVED = new Set([
  'auth', 'admin', 'api', 'users', 'oauth', 'health',
  'hermes', 'chat', 'sessions', 'assets', 'architecture',
])
```

`registry.test.ts` asserts no first-level node segment is in this set. (`architecture` is reserved only at depth 1 — `about-project/architecture` is fine, and the legacy `/architecture/:slug` route redirects there.)

### 6.3 Redirects

```ts
// src/resume/redirects.ts — old URL → new path. Append when you move a node.
export const REDIRECTS: Record<string, string> = {
  '/architecture': '/about-project/architecture',
  // '/architecture/:slug' handled by LegacyArchitectureRedirect
}
```

### 6.4 Why no infra change is needed

`deploy/gateway.yaml` ends with:

```yaml
      - match: /*
        upstream: web
        auth: public
```

Every path not claimed by `/oauth`, `/api`, `/users`, `/health`, `/hermes`, `/chat`, `/sessions`, `/admin` already reaches the SPA container publicly, and `web/nginx.conf` already does `try_files $uri $uri/ /index.html`. So `https://filenkov.store/experience/sber` works on the first deploy with **zero YAML and zero Go changes**. This is why §6.2's reserved list is the only routing constraint.

### 6.5 Scroll behaviour

* On node change: `window.scrollTo({ top: 0, behavior: 'auto' })` — **instant, never smooth**. A smooth scroll racing a fade-in is the classic source of motion sickness.
* On browser Back/Forward: restore the previous scroll position. Keep `Map<path, scrollY>` written in a `useEffect` cleanup and read on `POP` navigations (`useNavigationType()`).
* The one place `behavior: 'smooth'` is allowed is the owl button (§8.4), because that is the reader's explicit request to travel.

### 6.6 Document head

The registry knows every node at build time, so per-node head tags are trivial:

```ts
useEffect(() => {
  document.title = node.meta.seo?.title ?? `${node.meta.title} · Alexey Filenkov`
  setMeta('description', node.meta.seo?.description ?? node.meta.teaser)
  setLink('canonical', `https://filenkov.store/${node.path}`)
}, [node.path])
```

See §13.1 for the prerendering that makes these visible to crawlers.

---

## 7. Header

### 7.1 Contents

**Expanded** (`--header-h`, at the top of the page):

```
◆ Filenkov     Messanger · Storage · Hermes · Image · Chat · Admin     [ Sign in ]
                                                                     or  alexey  [Sign out]
```

* Brand mark + wordmark, links to `/`.
* **Service links** — the "refs to available services" from the brief, plus later hosts:

| Link | Href | Shown to |
|---|---|---|
| Messanger | `https://messanger.filenkov.store` | everyone, lock for guests |
| Storage | `https://storage.filenkov.store` | everyone, lock for guests |
| Hermes | `/hermes/chat` | everyone, lock for guests |
| RAG | `https://rag.filenkov.store` | everyone, lock for guests |
| Image | `https://image.filenkov.store` | `image` role `FAMILY` or `ADMIN` only, no lock |
| Chat | `https://llm.filenkov.store` | hub `global_role === 'ADMIN'` only, no lock |
| Admin | `/admin` | hub `global_role === 'ADMIN'` only |

Guests see Messanger / Storage / Hermes / RAG with a small `🔒` and `title="Sign in required"`. They are *not* hidden: the running house is part of the resume, and the gateway already redirects an unauthenticated navigation to `/auth?return_to=…`, so clicking one is a working sign-in path, not a dead end. Image, Chat, and Admin are hidden from people who would get a 403, because advertising that is noise. Chat is the llama.cpp server on `llm.filenkov.store` (`auth: admin`). The hub owl still calls LiteLLM internally.

* Right side: `Sign in` button, or `username` + `Sign out` (existing `useAuth` logic, unchanged).

**Condensed** (`--header-h-c`, when scrolling back up mid-page): brand mark only (no wordmark, no service links) + the sign-in control. Background `color-mix(in srgb, var(--paper) 82%, transparent)` with `backdrop-filter: blur(8px)`, hairline bottom rule.

The owl is **no longer in the header**. It moves to the hero slot (§8).

### 7.2 State machine

Three states on `document.documentElement[data-header]`:

| State | Entered when |
|---|---|
| `top` | `scrollY < 8` |
| `hidden` | scrolling **down**, cumulative down-delta > 24 px, and `scrollY > 96` |
| `condensed` | scrolling **up**, cumulative up-delta > 24 px, and `scrollY >= 8` |

Additional guards:

* **Hysteresis.** Ignore any frame where `|Δy| < 6px`, and require 24 px of accumulated travel in one direction before switching. Without this the header flickers on trackpad micro-scrolls — the single most common way this pattern is done badly.
* **Focus safety.** If `header.contains(document.activeElement)`, force `top`/`condensed` and never `hidden`. A keyboard user tabbing from the address bar must not be tabbing into an invisible element.
* **Overscroll.** Clamp `scrollY` to `>= 0` so iOS rubber-banding does not toggle states.
* **Menus.** If a header menu is ever added, freeze the state while it is open.

### 7.3 Implementation note — do not re-render on scroll

The scroll handler must not call `setState`. React re-rendering the tree at 60 fps is the fastest way to lose the smoothness this whole document is about.

```ts
// src/hooks/useScrollChrome.ts  — runs once, in Layout
useEffect(() => {
  const root = document.documentElement
  let last = window.scrollY, acc = 0, ticking = false
  const apply = () => {
    ticking = false
    const y = Math.max(0, window.scrollY)
    const d = y - last
    if (Math.abs(d) < 6) return
    acc = Math.sign(d) === Math.sign(acc) ? acc + d : d
    last = y

    root.dataset.owl = y > OWL_DOCK_AT ? 'docked' : 'hero'
    root.style.setProperty('--read', String(readProgress()))   // §8.5 ring

    if (y < 8)                      { root.dataset.header = 'top'; return }
    if (headerHasFocus())           { root.dataset.header = 'condensed'; return }
    if (acc > 24 && y > 96)         { root.dataset.header = 'hidden' }
    else if (acc < -24)             { root.dataset.header = 'condensed' }
  }
  const onScroll = () => { if (!ticking) { ticking = true; requestAnimationFrame(apply) } }
  window.addEventListener('scroll', onScroll, { passive: true })
  return () => window.removeEventListener('scroll', onScroll)
}, [])
```

Everything else is CSS:

```css
.header { position: fixed; inset: 0 0 auto; height: var(--header-h);
          transition: transform var(--dur) var(--ease), height var(--dur) var(--ease); }
[data-header='hidden']    .header { transform: translateY(-100%); }
[data-header='condensed'] .header { height: var(--header-h-c);
                                    background: color-mix(in srgb, var(--paper) 82%, transparent);
                                    backdrop-filter: blur(8px);
                                    border-bottom: 1px solid var(--rule); }
[data-header='condensed'] .brand-name,
[data-header='condensed'] .service-nav { opacity: 0; pointer-events: none; }
```

`passive: true` on the listener is mandatory — without it the browser cannot scroll on the compositor thread.

---

## 8. The owl

The brief: *"Owl must be on the top of the user window; when the user scrolls down the owl transforms into the button which routes to the top of the page."*

### 8.1 Two states

| State | Where | Size | Behaviour |
|---|---|---|---|
| `hero` | centred in the flow, between header and article | 128 px | Not a control. Plays the mood clip on mount, then rests. `aria-hidden`. |
| `docked` | `position: fixed; right: 24px; bottom: 24px` | 48 px visual / 56 px hit area | A real `<button>`: *"Back to top"*. |

Threshold `OWL_DOCK_AT = 180` px, i.e. once the hero owl has left the viewport. Hysteresis: dock at 180, undock at 140, so a reader hovering near the boundary does not see it flicker.

### 8.2 Mood clips — keep what exists

`HeaderMascot.tsx` already picks an SVG morph by auth state. Preserve it exactly, in the new component:

| Auth state | Clip | Read as |
|---|---|---|
| guest / on `/auth` | `calm-to-angry.svg` | "I don't know you yet" |
| signed in | `angry-to-calm.svg` | "recognised" |
| admin | `calm-to-kind.svg` | "hello, boss" |
| sign-out in progress | `calm-to-angry.svg`, then `onDeparted` fires the actual logout | the existing depart choreography, unchanged |

The `stripXml` + `innerHTML` injection and the `departNonce` replay mechanism carry over as-is; only the mount point moves.

Idle life: after the clip settles, a blink every 6–9 s (random) so the hero does not read as a dead PNG. One `<animate>` on the eyelid path, or a CSS `steps()` sprite. Suppressed under `prefers-reduced-motion` and when the tab is hidden (`document.visibilityState`).

### 8.3 The transform — FLIP

Use **one DOM element** for both states so the owl genuinely travels rather than cross-fading two owls.

```ts
// measured once on mount and on resize, not per frame
const heroRect = heroSlotRef.current.getBoundingClientRect()   // in-flow placeholder
const dockRect = { right: 24, bottom: 24, size: 48 }

// the element itself is position: fixed at the DOCK position, always.
// 'hero' state applies the inverse transform to put it visually in the hero slot:
const dx = (heroRect.left + heroRect.width / 2) - (window.innerWidth - 24 - 24)
const dy = (heroRect.top  + heroRect.height / 2) - (window.innerHeight - 24 - 24)
const s  = 128 / 48
root.style.setProperty('--owl-x', `${dx}px`)
root.style.setProperty('--owl-y', `${dy}px`)
root.style.setProperty('--owl-s', String(s))
```

```css
.owl { position: fixed; right: 24px; bottom: 24px; width: 48px; height: 48px;
       transform-origin: center;
       transition: transform var(--dur) var(--ease), opacity var(--dur) var(--ease);
       will-change: transform; }
[data-owl='hero']   .owl { transform: translate3d(var(--owl-x), var(--owl-y), 0) scale(var(--owl-s)); }
[data-owl='docked'] .owl { transform: translate3d(0, 0, 0) scale(1); }
```

Only `transform` animates → compositor-only, no layout, no paint. An empty `.owl-slot { height: 128px }` placeholder holds the hero space so nothing reflows when the owl detaches.

Note `dy` is measured in **viewport** coordinates and the hero slot scrolls away, so recompute `--owl-y` inside the same rAF as §7.3 while `data-owl === 'hero'` (it is one `getBoundingClientRect` on a 128 px box — cheap, and it only runs in the first ~180 px of scroll).

**Fallback if FLIP proves fiddly:** ship V1 as two elements cross-fading on the same `data-owl` attribute (hero owl `opacity/scale` out, corner button in). Same states, same CSS hooks, ~20 lines instead of ~60. Upgrade to FLIP once the rest is stable. Do not let the flourish block the release.

### 8.4 Docked button behaviour

* `<button type="button" aria-label="Back to top">` — a real button, not a div.
* Click: `window.scrollTo({ top: 0, behavior: reducedMotion ? 'auto' : 'smooth' })`, then move focus to the article `<h1>` (`tabIndex={-1}`) so keyboard users land where the eye lands.
* On arrival at the top, play `angry-to-calm` once as a small reward.
* Hidden entirely (`opacity: 0; pointer-events: none`) while `data-owl='hero'` — but kept in the DOM so the transform is continuous.
* Respect the safe area: `bottom: max(24px, env(safe-area-inset-bottom) + 8px)`.

### 8.5 Read-progress ring (optional, cheap, recommended)

A 2 px circular stroke around the docked owl showing progress through the current node:

```css
.owl-ring { stroke-dasharray: 151; stroke-dashoffset: calc(151 * (1 - var(--read))); }
```

`--read` is written by the same rAF handler in §7.3 (`scrollY / (scrollHeight - innerHeight)`, clamped 0–1). No extra listener, no React state. It gives the reader a sense of the node's length, which is exactly what a tree of unknown-depth articles otherwise lacks.

---

## 9. Content migration — every item in `data.ts` has a home

`src/resume/data.ts` is 341 lines and is the current source of truth. Nothing in it is discarded.

### 9.1 Root node — `nodes/node.tsx`

From `data.ts`: `profile` (name, title, years, english, lede, contacts).
Layout: kicker `RESUME`, `h1` name, role line, lede, a two-column contact block (Email / Telegram / GitHub), then the four branch cards. **No other content** — the root is a visit card, not a summary.

### 9.2 Experience

| Source in `data.ts` | New node |
|---|---|
| `experienceTree[0]` Sber | `experience/sber` — body: the 1M RUB/yr line, chips, what the role actually was |
| `…children[0]` 2× cheaper tokens (+ its child "Fewer loops, same quality") | `experience/sber/cheaper-tokens` — the two levels merge into one article; the child becomes a section |
| `…children[1]` Reports that remember | `experience/sber/report-memory` |
| `…children[2]` Tools as MCP | `experience/sber/mcp-tools` |
| `experienceTree[1]` AeroClub | `experience/aeroclub` |
| `…children[0]` Multi-label mail routing (+ child "Task-shaped confusion matrix") | `experience/aeroclub/mail-routing` |
| `…children[1]` NER at 80%+ | `experience/aeroclub/ner` |
| `…children[2]` Human-in-the-loop framework | `experience/aeroclub/human-in-the-loop` |
| `education[]` | `experience/education` |

Note the depth-4 accordion items in the current data (`exp-sber-graph`, `exp-aero-cm`) become **sections inside** their parent article rather than nodes. A node should be worth a click; a two-sentence detail is not.

### 9.3 Achievements

`achievements` (new writing) leads with the measured outcomes and, for each, one line on *how it was measured* — that is what makes a number credible to an interviewer:

* ~1M RUB/year saved at 1,000 reports/week (Sber)
* 60k → 30k tokens per report (Sber)
* 85% precision / 80% recall on unseen mail, prompt-only (AeroClub)
* NER > 80% on new mail (AeroClub)

`achievements/projects` receives `projectTree()` verbatim: `auth-hub`, `storage`, `llm`, `resume-studio`, and `hermes` with `visibleWhen: 'signed-in'` (replacing the current `showHermes` boolean argument — the registry's visibility filter does the same job declaratively).

### 9.4 About this project → architecture

`architectureSolutions[]` (10 entries) does **not** become 10 folders. It stays a data array, now at `nodes/about-project/architecture/solutions.ts`, and the node renders:

* the `architecture.svg` map in a `--paper-raised` figure, horizontally scrollable, `loading="lazy"`;
* a grid of solution cards linking to `/about-project/architecture/<slug>`;
* a **parameterised child route** — one `node.tsx` handling `:slug`, i.e. `pages/Architecture.tsx` moved and re-skinned. The registry gains one flag, `dynamicChild: true`, so the spine can render `Architecture › Auth Gateway` for a slug that has no folder.

This keeps 10 near-identical stubs out of the tree while preserving their URLs. When a solution page grows real content, promote it to its own folder and add a redirect.

`about-project/why-a-tree` and `about-project/site-stack` are new writing.

### 9.5 For interviewer

`stackTree[]` maps to `for-interviewer/stack/{ai-agents,backend,infra}`; the second-level items (`stack-graphs`, `stack-mcp`, `stack-rag`, `stack-prompt`, `stack-data`, `stack-auth-impl`, `stack-edge`, `stack-roles`) become sections inside those three articles, with the chip lists preserved.

`for-interviewer/logistics` collects the facts a recruiter needs in one place: 5 years' experience, English B1, location, notice/availability, contacts, and a link to a downloadable PDF resume (§13.2).

`for-interviewer/how-i-work` is new writing.

### 9.6 A node module, for reference

```tsx
// src/resume/nodes/experience/sber/meta.ts
import type { NodeMeta } from '../../../types'
const meta: NodeMeta = {
  title: 'Sber',
  meta: '2023 — 2026 · 3 years',
  teaser: 'Middle+ AI Agents. Automated reporting: ~1M ₽/year saved at 1,000 reports a week.',
  order: 10,
}
export default meta
```

```tsx
// src/resume/nodes/experience/sber/node.tsx
import { Figure, Chips, Callout } from '../../../../components/article'

export default function SberNode() {
  return (
    <>
      <p className="lede">Reporting that used to take an analyst a day now runs on a graph.</p>
      <p>…</p>
      <Chips items={['LangGraph', 'FastAPI', 'PostgreSQL', 'Kafka', 'Faiss', 'RAG']} />
    </>
  )
}
```

The node body renders **only the article content**. The kicker, `h1`, meta line, spine, branch cards and skeleton are all supplied by `NodeView` from `meta`. A node author never writes chrome.

Shared article primitives live in `src/components/article/`: `Chips`, `Figure`, `Callout`, `Metric`, `Timeline`, `DefinitionList`. They are the *only* sanctioned way to add visual variety — this is what stops thirty nodes from drifting into thirty different layouts.

---

## 10. Performance budget

| Metric | Budget | How |
|---|---|---|
| Initial JS, gzip | **≤ 95 KB** | React 19 + router ≈ 58 KB, shell ≈ 25 KB, all `meta.ts` ≈ 4 KB |
| Per-node chunk, gzip | **≤ 12 KB** | prose + article primitives (already in the shell) |
| LCP (4G, mid Android) | **< 1.5 s** | self-hosted fonts, no blocking third party, hero owl is inline SVG |
| CLS | **0** | fixed owl slot height, reserved skeleton min-height, font metric overrides |
| INP | **< 100 ms** | no React re-render on scroll; navigation is a prefetched chunk swap |
| `architecture.svg` | never in the initial load | lives under a depth-2 node, `loading="lazy"` |

### 10.1 Fonts — the biggest single win

`index.html` currently pulls Fraunces (2 weights, variable optical size) and Public Sans (4 faces) from `fonts.googleapis.com`. That is a DNS + TLS + CSS + font round trip to a third party before any text paints.

Self-host: subset to `latin` + the few punctuation glyphs used, `woff2`, `font-display: swap`, `<link rel="preload">` the two faces actually used above the fold (Fraunces 650, Public Sans 400). Add metric overrides so the fallback does not shift:

```css
@font-face {
  font-family: 'Public Sans Fallback';
  src: local('Arial');
  size-adjust: 107%; ascent-override: 92%; descent-override: 24%; line-gap-override: 0%;
}
```

Expect ~250–400 ms off first paint on a cold 4G connection, and CLS from font swap goes to zero.

### 10.2 Images

The owl SVGs are injected as raw markup (`?raw`) — that is correct and stays. `architecture.svg` at 128 KB is imported `?url` today, so it is already an asset rather than inline; keep that, add `loading="lazy" decoding="async"`, and give the `<img>` explicit `width`/`height` to reserve its box.

### 10.3 Bundle guard

Add to CI (or at least to `npm run build`): fail if `dist/assets/index-*.js` exceeds the budget. `rollup-plugin-visualizer` in `--report` mode for the occasional look.

---

## 11. Accessibility

* **Landmarks:** `<header>`, `<nav aria-label="Primary">` (services), `<nav aria-label="Resume path">` (spine), `<main>`, `<footer>`. One `h1` per node, supplied by `NodeView`.
* **Skip link** as the first focusable element: *"Skip to article"* → `#node-article`. It becomes visible on focus. Mandatory here, because the header holds several links before the content.
* **Node change announcement:** a visually-hidden `aria-live="polite"` region emits the node title once the body has resolved. Focus moves to the `h1` (`tabIndex={-1}`), which is also what the owl button does.
* **Spine:** `<ol>`, current item `aria-current="page"` and not a link.
* **Branch cards:** `<a>` elements in a `<ul>`. The whole row is the link; the `→` is `aria-hidden`.
* **Header hiding:** never `display: none` and never `inert`. It is translated off-screen but still focusable, and `focusin` forces it back into view (§7.2).
* **Contrast:** all text ≥ 4.5:1 after the `--accent-text` split (§3.2). Chips (`--ink-muted` on `--chip`) must be re-checked after the palette edit — if they fail, darken the chip text to `--ink`.
* **Hit targets:** ≥ 44 × 44 px for the owl button, branch cards, spine dots, and the mobile breadcrumb chips.
* **Reduced motion:** §3.5.
* **Keyboard-only walkthrough** is part of the definition of done: Tab from the address bar must reach skip link → header → spine → article → branch cards, in that visual order, with a visible ring at every stop.

---

## 12. Testing

Existing stack: `vitest` + `@testing-library/react` + `jsdom`. Everything below fits it; no new dependencies.

**`registry.test.ts` — the invariants that make convention-over-configuration safe:**

1. every `meta.ts` has a sibling `node.tsx`, and vice versa;
2. every non-root node's parent path exists;
3. every path segment matches `/^[a-z0-9]+(-[a-z0-9]+)*$/`;
4. no depth-1 segment is in `RESERVED` (§6.2);
5. sibling `order` values are unique;
6. `depth <= 4`;
7. every `teaser` is non-empty and ≤ 120 chars (branch cards must not wrap to four lines);
8. the root node exists at path `''`.

**Component tests:**

* `Spine.test.tsx` — renders the ancestor chain for a deep path; current item has `aria-current`; ancestors are links.
* `BranchCards.test.tsx` — children sorted by `order`; a `signed-in` node is absent for a guest and present for a signed-in user; siblings render; the root shows no "up" link.
* `NodeRoute.test.tsx` — known path renders; unknown path redirects to `/`; `/architecture/auth-gateway` redirects to `/about-project/architecture/auth-gateway`; a `signed-in` node redirects a guest.
* `Suspense.test.tsx` — a slow `load()` shows the skeleton *with the real title*; a rejecting `load()` shows the error boundary with a Reload button.
* `useScrollChrome.test.ts` — drive `window.scrollY` + dispatch `scroll`, flush rAF, assert `data-header` transitions `top → hidden → condensed → top`; assert a 4 px jitter changes nothing; assert focus inside the header prevents `hidden`.
* `Owl.test.tsx` — `data-owl` flips at the thresholds; the docked owl is a `<button>` with the right label; click calls `scrollTo` and moves focus.

**Manual checklist before merge:** keyboard walkthrough (§11), a 3× CPU-throttled + Slow 4G run of `Start → Experience → Sber` watching for skeleton flash, `prefers-reduced-motion` on, 360 px viewport, and Lighthouse ≥ 95 on Performance and Accessibility.

---

## 13. Two things worth deciding before Phase 1

### 13.1 Crawlability

A resume site's most important reader may be a recruiter's Google search, and an SPA that renders everything in JS shows a crawler an empty `<div id="root">`. The registry knows **every URL at build time**, which makes this easy to fix:

* add a prerender step to `npm run build` that walks the registry, renders each node path in headless Chromium (`puppeteer-core`, or the Chromium the repo already has available for Playwright), and writes `dist/<path>/index.html` with the real markup and the correct `<title>` / `<meta description>`;
* nginx's `try_files $uri $uri/ /index.html` then serves the real HTML file when it exists and falls back to the SPA otherwise — **no nginx change needed**;
* generate `dist/sitemap.xml` from the same registry, and a `robots.txt`.

This is ~60 lines of build script and roughly doubles the value of the site as a resume. Recommended for Phase 4, not a launch blocker.

### 13.2 A downloadable PDF

Recruiters ask for a file. `for-interviewer/logistics` should link to one. Given the Resume studio project already generates tailored PDFs with XeLaTeX, the cheapest path is a static `public/alexey-filenkov.pdf` kept up to date by hand, with the studio wired in later. Decide whether it is in scope.

---

## 14. Implementation phases

Each phase ends green (`npm run lint && npm test && npm run build`) and is independently shippable.

**Phase 0 — foundation (no visible change)**
Split `index.css` into `src/styles/*` · add tokens, `--accent-text`, focus ring · delete the background gradient/rules · self-host fonts · delete `--shadow` usages.

**Phase 1 — tree engine (behind the existing page)**
`resume/types.ts`, `registry.ts`, `redirects.ts` · `registry.test.ts` with all 8 invariants · `components/tree/{Spine,BranchCards,NodeView,NodeSkeleton,NodeBoundary}.tsx` · `hooks/useNodePrefetch.ts` · routes in `App.tsx` · seed with **three** nodes only (root, `experience`, `experience/sber`) to prove the mechanism end to end.

**Phase 2 — content migration**
Write every node folder per §9 · move `architecture.svg` and `solutions.ts` · delete `RevealTree.tsx`, `pages/Home.tsx`, `pages/Architecture.tsx`, `resume/data.ts` and their tests · add `article/` primitives.

**Phase 3 — chrome**
`useScrollChrome.ts` · new `Layout.tsx` with the three header states and service links · `HeaderMascot.tsx` → `components/owl/Owl.tsx`, two-element version first · skip link, live region, focus management.

**Phase 4 — polish**
FLIP owl morph · read-progress ring · idle prefetch of children · scroll restoration on Back · reduced-motion pass · prerender + sitemap (§13.1) · Lighthouse and the manual checklist.

Phases 0 and 1 are the ones that must not be rushed; 2 is volume work; 3 and 4 are where the site starts to feel expensive.

---

## 15. Open questions

1. **Owl corner.** Bottom-right is specified (§8.1) — thumb-reachable, conventional, clear of the condensed header. Confirm, or say if you want top-right.
2. **Guest view of service links.** Specified as visible-with-lock (§7.1). The alternative is hiding them from guests entirely.
3. **Achievements content.** §9.3 uses the four numbers already in `data.ts`. Anything else measurable worth adding — team size led, latency, uptime, model eval scores?
4. **"How I work"** is new writing with no source in `data.ts`. Sketch it, or drop the node for now?
5. **PDF resume** — §13.2.
6. **Analytics.** None today. If you want per-node read counts (genuinely useful: it tells you which branch recruiters actually open), it needs a decision — self-hosted Plausible/Umami behind the gateway, or nothing.

---

## 16. What shipped

The tree is live. 30 nodes exist (root + 29), one folder each under `src/resume/nodes/`, discovered by `registry.ts` with `import.meta.glob`; `registry.test.ts` enforces all 8 invariants of §12. Routes are §6.1 as written. `src/index.css` is split, fonts are self-hosted, `RevealTree.tsx`, `HeaderMascot.tsx`, `pages/Home.tsx`, `pages/Architecture.tsx`, `resume/data.ts` and their tests are deleted; `pages/Auth.tsx` and `pages/Admin.tsx` are untouched. §6.4 held: **no Go, gateway or `deploy/gateway.yaml` change was needed or made**, and `web/nginx.conf` only gained cache headers for `/assets/` and `/fonts/` — its `try_files` rule is unchanged. `vite.config.ts` gained the §13.1 sitemap/`robots.txt` generator and the §10.3 bundle guard, which fails the build over 95 KB gzipped of initial JS; it is 88.5 KB today.

Where the implementation departs from the design:

* **§8.3 the owl — cross-fade, not FLIP.** Shipped as the sanctioned two-element fallback at the end of §8.3. The FLIP transform has to be re-measured on every scroll frame while the hero slot scrolls away, which fights the 240 ms transform transition: the owl trails the page. See the comment at the top of `src/components/owl/Owl.tsx`.
* **§13.1 crawlability — half done.** `sitemap.xml` and `robots.txt` are generated from the node folders at build time (signed-in nodes excluded). Headless-Chromium prerendering was **not** done — it needs a new dependency. Still open.
* **§13.2 PDF resume — declined.** No PDF exists in the repo. `for-interviewer/logistics` deliberately explains why there is no download rather than linking a dead file.
* **§5.3 `NodeMeta` gained three fields.** `heading` — the `h1` when it should differ from the short navigation label (the root is `Start` in the spine but `Alexey Filenkov` as the `h1`); `kicker` — the small line above the `h1`, defaulting to `Resume`; and `dynamicChild`, which is a resolver object (`{ resolve(slug) }`) rather than the boolean flag §9.4 assumed, so a dynamic child can supply its own title, teaser and `<title>`.
* **§3.6 stylesheets — seven files, not six.** `fonts.css` was added for the self-hosted faces and is imported first in `main.tsx`.
* **§7.3 scroll handler — the jitter floor gates the header only.** The 6 px floor applies to the header state machine. The owl dock state and the read-progress ring update on every frame, because the floor would otherwise swallow the 1 px that carries the reader across the 180 px dock threshold.

Open questions (§15), resolved:

1. **Owl corner** — built as specified, bottom-right.
2. **Guest service links** — built as specified, visible with a lock.
3. **Achievements content** — the four numbers already in `data.ts`, nothing added.
4. **"How I work"** — written.
5. **PDF resume** — declined, see above.
6. **Analytics** — not done. Still open.

The prose in `for-interviewer/how-i-work` and `for-interviewer/logistics` is newly written and has **not** been reviewed by Alexey.
