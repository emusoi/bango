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

`order` lists section ids, as a preference. Sections not named follow in the order
given, and an id naming a section that is not present this time is ignored — a
producer builds its sections from the world and orders them from a fixed list.

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

Colour follows `kind` as well: a `count` is the accent, a `ref` reads as a
reference, a `time` is quiet, a section label is quiet and italic, and a `dim`
row is quiet throughout. All of it comes from the terminal's own palette through
adaptive colours, so a light theme and a dark one both work, and `--plain`
turns it off entirely.

## Action

| field | |
|---|---|
| `key` | the key that runs it, unique in the panel |
| `label` | what to call it |
| `verb`, `args` | the command, in drive mode only |
| `input` | prompt for one line; it becomes `{input}` |
| `choices` | offer a list; the pick becomes `{choice}` |
| `confirm` | ask before running |
| `panel` | this action's command prints another panel; render it instead of refreshing |
| `global` | applies to the screen, not a row |
| `retry` | `[{when, label, verb, args}]` — offered when a refusal contains `when` |
| `help` | one sentence, shown under `?` |

`/`, `?`, `q`, `esc`, `j`, `k`, `g`, `G` and space are reserved by renderers and
rejected at validation.

A key must be unambiguous **for a row**, not across the panel: two actions may
share a key when no single row offers both, which is how `D` can mean *forget the
stack* on one row and *delete the worktree* on another. A global action's key
must not collide with any row's actions.

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

`--watch N` re-runs the producer every N seconds here too, so a panel whose
world changes without anybody pressing a key keeps up with it. A redraw restores
the cursor by row id and never changes the mode, so it cannot arrive in the
middle of a prompt and take it away.

If an action's own output parses as a panel, that panel is **pushed** instead:
the screen becomes the new one and `q` or `esc` pops back to where you were.
`panel` on the action declares that it does this, so a reader of the document
knows without running it. That is the whole of "panels of panels" — one
mechanism, no second kind of action.

An action with `choices` offers them as a list first, and the pick becomes
`{choice}`. An action whose command fails shows what it said, and if a `retry`
matches that refusal it is offered on `r` — a refusal that names its own fix.

> Argv is executed only when the user typed the program that produced it. There
> is no flag to change that, because a flag to disable it is a flag someone puts
> in a script.

`{row}`, `{input}` and `{choice}` are substituted as whole argv elements. There
is no shell in that path.

## Tabular input

`bango table` is a producer for tools that have no panel of their own. It reads
tab-separated or JSON lines and prints a document; it renders nothing, so it
composes with everything above. Values are cut to one line, because a field is
one line, and a repeated id is suffixed rather than refused, because the column
a reader wants to see is rarely unique.

Every panel it builds declares one action, `pick`, so that select mode has
something to pick with — a panel with no actions cannot be chosen from at all.
Without `--run` that action's verb is `true`, which does nothing in drive mode
and is never executed in select mode.

## Remote

A panel is a document, so the read side needs nothing:

```sh
ssh box mytool panel dash | bango
```

That is select mode, which executes nothing, so a panel from a machine you do
not control can describe whatever actions it likes and none of them run.

To act, the actions have to run where the panel came from. A transport prefix
applies to **both** the producer and its actions:

```sh
bango --via-ssh fedora -- mia api dashboard --bango
bango --via 'docker exec -i web' -- mytool panel dash
bango --via 'kubectl exec pod --' -- mytool panel dash
bango --via 'mia run monduli' -- loco panel confirm --json
```

`--via` is an **argv prefix**: the verb and its arguments are appended as
separate arguments and nothing is quoted, which is right for `docker exec`,
`kubectl exec` and anything else that takes an argv.

`--via-ssh` is different because sshd runs what it receives through the remote
login shell. bango shell-quotes every element and sends one string, so a
finding whose message is `two words; rm -rf /` arrives as one argument and
nothing else happens. It also passes `ControlMaster=auto` and
`ControlPersist=60s`, because drive mode opens a connection per action and the
second one should be free.

> The producer and its actions always share a transport. There is no way to
> render a remote panel and run its actions locally: stdin is select-only, and
> `--via` covers both halves. Actions run where the panel came from, so a
> compromised producer can only choose argv that runs on the machine you already
> asked.

A tool that knows about remoteness itself needs none of this — `mia api
dashboard --host fedora` could emit a panel whose verbs already say `mia run
fedora …`, and bango would be none the wiser. That is the better shape when a
producer has somewhere to put it; `--via` is what makes every other tool work
today.

## A browser

A browser cannot run a subprocess or read a pipe, so it needs a server. That
server is `bango --serve`, not anything the producer has to grow.

```sh
bango --serve 127.0.0.1:0 --watch 5 -- mia api dashboard --bango
http://127.0.0.1:59065/?t=31c073d0…
```

| | |
|---|---|
| `GET /` | the page |
| `GET /panel` | `{revision, readOnly, panel}` |
| `GET /events` | server-sent events: one `id: <revision>` + `data: {panel}` per change |
| `POST /act` | `{action, row, input}` — runs it, refreshes, returns the new panel |

**Server-sent events, not websockets.** The server→client side is a one-way
stream of panels, which is exactly what SSE is, in one HTTP request, with
automatic reconnection and `Last-Event-ID` for free and no library on either
end. The client→server side is a discrete command, which is a POST. A websocket
would add framing, ping/pong and reconnection logic to buy bidirectional
streaming that nothing here needs. If a client ever has to push continuously,
that is when to revisit it.

`--watch N` re-runs the producer every N seconds. Without it a panel changes
only when an action changes it. A producer that starts failing does not silently
freeze the page: the last good panel stays, and what the producer said arrives
with it and clears when it recovers.

### What guards it

A local HTTP server is reachable by any page in the browser, so it is not
trusted by origin:

- It binds **loopback only**, on an ephemeral port by default.
- Every request carries a 32-byte token, printed once in the URL and written to
  `<cache>/bango/serve.token` with mode 0600. It is compared in constant time.
- The `Host` header must be loopback and the expected port, which is what stops
  DNS rebinding.
- Any `Origin` that is not this server is refused, and no CORS headers are ever
  sent, so a cross-origin read fails twice over.
- The page is served under a Content-Security-Policy that allows it to talk to
  nothing but its own origin.
- `--read-only` serves a panel whose actions are refused with 403.
- An action is refused unless the panel offers it **on that row**, so a client
  cannot post a verb the screen never showed, and `{choice}` must be one the
  action listed, so it cannot post an argument the screen never showed either.
- The address is checked before the socket is opened, so `--serve` cannot be
  talked into a public interface.

Actions run under the same rule as drive mode: the user named the producer on
the command line, and the server only runs what that producer's panel declares.
A panel arriving on stdin is never served with actions enabled.

## Streaming

A producer may emit newline-delimited panels; a renderer redraws on each. This
works unchanged over ssh, because it is a pipe. A renderer decodes a panel at a
time rather than reading to the end, so the first one is drawn while the
producer is still talking and a producer that never closes still shows
something. A malformed document is reported and the last good panel stays on
screen; the stream is not resumed, because a half-read document has no place to
resume from.

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

## The window

The title, the subtitle and the hints are pinned; the rows between them are a
window on the panel. It holds still while the cursor moves inside it and follows
only when the cursor would leave, so moving one row never shifts the whole list.
Lines a renderer adds below the panel — a filter, a prompt, a notice — are
counted against the height first, because they take room from the same screen.
A renderer that draws into something already scrollable, such as a buffer or a
page, has no window and draws every row.

Below about sixty columns a panel with four fields is unreadable whatever a
renderer does. Producers should keep to three or four fields.

## Filter

The haystack is the row id, every field value and the note, lowercased. All
whitespace-separated terms must match, in any order. A parent whose child
matches stays; a child whose parent matches does not. Sections with no surviving
rows disappear. Counts in the subtitle are **not** recomputed: that number is the
producer's answer about the world, not about the filter.

## Validation

A panel may have no sections at all: that is an empty state, and `empty` is what
it renders. Every failure names its path. `bango`, `id`, section ids unique, row
ids unique across the panel, at least one field per row, field names unique
within a row, known kinds and marks, `actions` naming real actions, keys unique
and unreserved, every action and every one of its retries requiring a `verb`,
every retry requiring the `label` it is offered under, every `target` naming a
row that is on the panel,
nesting at most 8, and no key ambiguous on any one row.

## Exit codes

| | |
|---|---|
| 0 | a choice was made; it is on stdout |
| 1 | nothing to show, or no row matched |
| 2 | the panel was invalid, or its version is unknown; `check` found something |
| 130 | cancelled |

130 matches fzf, because people pipe both in one script.

## Limits

Widths are counted in terminal cells, so a glyph that occupies two columns is
measured as two and a combining mark as none. Truncation counts the same way and
never cuts a value that already fits. A renderer that cannot measure cells
renders CJK narrow and misaligns every column to its right; measuring is the
conformance bar, and `fixtures/wide` is what checks it.

## The schema

`panel.schema.json` states this document in JSON Schema, and `bango schema`
prints it. It is generated from nothing — it is written — so the renderer's
tests hold it to the vocabulary here: the marks, the kinds, the reserved keys
and the document version are compared against the code, and a schema that has
drifted fails the build. Two things it cannot say are the nesting limit of 8 and
that a key must be unambiguous per row; `bango check` says both.

## Conformance

`fixtures/` holds panels and their expected renderings at fixed widths. A
renderer is correct when it reproduces them. A sentence in this document with no
fixture behind it has not been checked.

## Where a renderer draws

A renderer reads the panel from stdin and draws on the terminal, which means
those cannot be the same stream. When stdin or stdout is a pipe, a renderer
opens `/dev/tty` for its own input and output — the way fzf does — so
`mytool --panel | bango` works from a shell, inside a terminal buffer, and
anywhere else a controlling terminal exists.

Where there is no controlling terminal at all, it says so and names `--print`,
rather than drawing nothing and waiting for a keystroke that cannot arrive.
