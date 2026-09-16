# bango.nvim

A float, a terminal, and the real `bango` binary inside it.

```lua
require("bango").pick {
  cmd = { "loco", "panel", "confirm", "--json" },
  on_choice = function(choice)
    vim.cmd.edit(choice.row)
  end,
}

require("bango").pick {
  cmd = { "mia", "api", "dashboard", "--bango" },
  drive = true,
}
```

`drive = false` (the default) pipes the producer into select mode: bango draws,
you pick, nothing is executed, and `on_choice` gets `{action, row, input}`.
`drive = true` hands the producer to bango, which runs its actions itself.

`ascii`, `args`, `width` and `height` are the other options. `args` goes to
bango, so `args = { "--via-ssh", "fedora" }` picks from another machine.

## Why a terminal and not a Lua renderer

fzf.vim is a terminal wrapper rather than a reimplementation, and this is the
same bet: one renderer, one set of behaviours, no second layout algorithm to
keep in step.

A native Lua renderer earns its place the moment a picker has to **preview
inside the host editor while the cursor moves** — a hunk shown in the main
window, a file opened with its own treesitter and LSP, a jump that leaves the
picker open. A subprocess can print a choice when it exits; it cannot drive the
editor on every keystroke.

Until a caller needs that, this is forty lines instead of four hundred.

## Two renderers

`require("bango").pick{…}` runs the real binary in a terminal float. One
renderer, no second layout algorithm, and the right answer when all you need is
a choice.

`require("bango").panel{panel = …}` draws the same document in Lua. It exists
for the one thing a subprocess cannot do: **preview inside the host editor while
the cursor moves**. `on_move(row, back)` is called on every selection change —
open a file, draw something, then call `back()` to return focus.

```lua
local bango = require "bango"
local panel = bango.read({ "mytool", "panel", "hunks", "--bango" }, cwd)
bango.panel {
  panel = panel,
  on_move = function(row, back) show(row.id); back() end,
  on_choice = function(choice) act(choice) end,
}
```

It shares the layout algorithm with the Go renderer, and the fixtures are run
against both — a rule that only survives because the fixtures exist.
