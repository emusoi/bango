# bango, version 1

A tool describes a screen as JSON. A renderer draws it. The tool's actions come
along as data.

## The document

```json
{
  "bango": 1,
  "id": "confirm",
  "title": "loco · #3386",
  "subtitle": "4/9 read · 2 to confirm",
  "empty": "nothing waiting on you",
  "hints": ["⏎ open", "y confirm"],
  "order": ["waiting", "open"],
  "sections": [{ "id": "waiting", "label": "to confirm", "rows": [] }],
  "actions": {}
}
```

`bango` is required and must equal 1. A renderer that meets a version it does not
know says so and exits; it never renders a guess.

`id` identifies the screen so a renderer can remember cursor, filter and folds
per panel. `[A-Za-z0-9._:/-]`, no spaces.

`order` lists section ids. Sections not named follow in the order given.

## Section

`id`, `label`, `rows`, `collapsed`. Grouping and ordering are the producer's
answer, not something a renderer re-derives.

## Row

| field | |
|---|---|
| `id` | stable, unique across the whole panel including children |
| `mark` | a state name from the vocabulary, never a character |
| `fields` | named values, in display order, at least one |
| `note` | one short trailing word |
| `actions` | names of the actions legal on this row |
| `facts` | detail lines for the selected row |
| `preview` | lines for a preview pane |
| `children` | nested rows, at most 8 deep |
| `dim` | draw faded |
| `target` | the id an action uses instead of `id` |

## Field

`{"name": "where", "value": "src/auth.ts:42", "kind": "path"}`

`kind` is semantic. A renderer decides presentation from it.

| kind | truncation | alignment |
|---|---|---|
| `text` (default) | tail | left |
| `path` | middle, keeping the basename whole | left |
| `count` | never | right |
| `time` | never | left |
| `ref` | only as a last resort | left |

Values are one line and unpadded. Padding, width and alignment belong to the
renderer, which is the only party that knows how wide the window is.

## Marks

| mark | meaning | unicode | ascii |
|---|---|---|---|
| (absent) | no state | | |
| `here` | where you are now | ▸ | > |
| `new` | not started | ○ | o |
| `working` | in progress | ◐ | % |
| `waiting` | waiting on a person | ⏎ | ! |
| `done` | finished, read, confirmed | ✓ | x |
| `dirty` | has uncommitted work | ● | * |
| `blocked` | cannot proceed | ⨯ | X |
| `detached` | lost its anchor | ⚠ | ~ |

A renderer may express a mark with colour or weight as well, never with colour
alone, and never with a glyph outside the set it declares.

## Action

| field | |
|---|---|
| `key` | the key that runs it, unique in the panel |
| `label` | what to call it |
| `verb`, `args` | the command, in drive mode only |
| `input` | prompt for one line; it becomes `{input}` |
| `choices` | offer a list; the pick becomes `{choice}` |
| `confirm` | ask before running |
| `panel` | open another panel instead of running a command |
| `global` | applies to the screen, not a row |
| `retry` | `[{when, label, verb, args}]` — offered when a refusal contains `when` |
| `help` | one sentence, shown under `?` |

`/`, `?`, `q`, `esc`, `j`, `k`, `g`, `G` and space are reserved by renderers and
rejected at validation.

## The two modes

**select** — the default, and the only behaviour for a panel arriving on stdin,
from a file, or from anywhere the user did not name. The renderer draws, the
user picks, one line is printed and it exits. **Nothing is executed.**

```json
{"action":"send_back","row":"t7","input":"the test still fails"}
```

**drive** — `bango -- mytool panel dashboard`. The renderer runs the named
command, renders the result, runs that panel's actions, and re-runs the producer
after each one.

> Argv is executed only when the user typed the program that produced it. There
> is no flag to change that, because a flag to disable it is a flag someone puts
> in a script.

`{row}`, `{input}` and `{choice}` are substituted as whole argv elements. There
is no shell in that path.

## Streaming

A producer may emit newline-delimited panels; a renderer redraws on each. A
malformed document is reported and the last good panel stays on screen.

On redraw the cursor is restored by row id. If that row is gone it holds its
index, clamped. Filter text, folds and preview scroll survive. The cursor never
moves for any other reason, and a redraw never steals focus.

## Layout

1. Columns are the union of field names, in first-seen order across visible rows.
2. A column's natural width is the widest value among them.
3. The budget is the window width minus the gutter (3) minus the note column
   (natural, capped at 20) minus one space between columns.
4. If the natural widths fit, the widest elastic column takes the remainder.
5. Otherwise shrink the widest of the text and path columns, one column at a
   time, until it fits. `count` and `time` never shrink. `ref` shrinks only when
   everything else is at its floor.
6. Floors: text 6, path 12, ref 8.

Widths are computed from visible rows, so folding changes the layout.

Below about sixty columns a panel with four fields is unreadable whatever a
renderer does. Producers should keep to three or four fields.

## Filter

The haystack is the row id, every field value and the note, lowercased. All
whitespace-separated terms must match, in any order. A parent whose child
matches stays; a child whose parent matches does not. Sections with no surviving
rows disappear. Counts in the subtitle are **not** recomputed: that number is the
producer's answer about the world, not about the filter.

## Validation

Every failure names its path. `bango`, `id`, `sections`, section ids unique, row
ids unique across the panel, at least one field per row, field names unique
within a row, known kinds and marks, `actions` naming real actions, keys unique
and unreserved, `args` requiring a `verb`, `verb` and `panel` mutually exclusive,
`order` naming real sections, nesting at most 8.

## Exit codes

| | |
|---|---|
| 0 | a choice was made; it is on stdout |
| 1 | nothing to show, or no row matched |
| 2 | the panel was invalid, or its version is unknown |
| 130 | cancelled |

130 matches fzf, because people pipe both in one script.

## Limits

Widths are counted in runes. Wide characters and combining marks are not
measured, and a panel full of CJK will render narrow. That is a known v1 limit,
not a design position.

## Conformance

`fixtures/` holds panels and their expected renderings at fixed widths. A
renderer is correct when it reproduces them. A sentence in this document with no
fixture behind it has not been checked.
