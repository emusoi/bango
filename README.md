# bango

A tool describes a screen as JSON. bango draws it — in a terminal, in Neovim, in
a browser — and the tool's own actions come along as data.

```sh
mytool status --bango | bango
git-branches | bango
bango -- mia panel dashboard
```

`bango` is Swahili for a signboard: the thing you pin notices to.

## Install

```sh
go install github.com/emusoi/bango/cmd/bango@latest
```

The renderer is one binary. A producer needs nothing installed — it emits JSON,
and `SPEC.md` is the whole contract.

## Twenty lines of shell

```sh
#!/bin/sh
git for-each-ref --sort=-committerdate refs/heads/ \
  --format='%(refname:short)%09%(committerdate:relative)%09%(subject)' |
jq -Rs '{
  bango: 1, id: "branches", title: "branches",
  sections: [{ id: "all", label: "recent first",
    rows: (split("\n") | map(select(length > 0)) | map(split("\t") | {
      id: .[0],
      fields: [
        {name:"branch", value:.[0], kind:"ref"},
        {name:"age",    value:.[1], kind:"time"},
        {name:"subject",value:.[2]}],
      actions: ["checkout","delete"]})) }],
  actions: {
    checkout: {key:"⏎", label:"checkout", verb:"git", args:["switch","{row}"]},
    delete:   {key:"dd", label:"delete",  verb:"git", args:["branch","-d","{row}"],
               confirm:"delete {row}?"}}}'
```

```sh
$ ./git-branches | bango          # picks, prints the branch, runs nothing
$ bango -- ./git-branches         # picks, and runs the checkout
```

## Without writing a producer

A tool that already prints columns does not need one. `bango table` turns
tab-separated or JSON lines into a panel, so anything with machine-readable
output gets a screen in one pipe:

```sh
docker ps --format '{{.Names}}\t{{.Status}}\t{{.Image}}' |
  bango table --columns name,status,image:ref --run 'docker logs {row}' | bango

kubectl get pods -A -o json | jq -c '.items[] | {pod:.metadata.name, ns:.metadata.namespace}' |
  bango table --section ns | bango
```

`--columns` names them in order, each optionally `name:kind`. Without it,
tab-separated input is numbered and JSON lines take their keys, sorted.
`--key` picks the column holding each row's id, `--section` groups by a column
and shows it as the heading rather than on every row, and `--run` is what a pick
executes, with `{row}` standing for the id. It prints a document and renders
nothing, so it pipes into `bango`, into a file, or over ssh like any other panel.

It is the shallow end. A panel with several actions, confirmations, retries or
marks is a producer, and a producer is usually a dozen lines of shell.

## The two modes

**select** is the default and the only behaviour for anything arriving on stdin.
bango draws, you pick, it prints the choice and exits. It executes nothing, so a
panel from a pipe, a file or an agent is safe to render.

**drive** — `bango -- CMD` — runs the command you named, renders it, and runs its
actions, re-running the producer after each one.

Argv is executed only when you typed the program that produced it.

## Remote

```sh
ssh box mytool panel dash | bango           # read it; nothing can run
bango --via-ssh fedora -- mia api dashboard --bango
bango --via 'docker exec -i web' -- mytool panel dash
```

A transport covers the producer and its actions together, so what you act on
runs where the panel came from. `--via` passes argv straight through;
`--via-ssh` shell-quotes every argument, because sshd runs what it gets through
a shell.

## In a browser

```sh
bango --serve 127.0.0.1:0 --watch 5 -- mia api dashboard --bango
http://127.0.0.1:59065/?t=31c073d0…
```

The same document, rendered as a page, updating over server-sent events, with
the panel's actions as buttons. Loopback only, a token on every request, `Host`
and `Origin` both checked, and `--read-only` when you want the screen without
the verbs.

## Writing a producer

Any language. The document is the contract and `jq` is a perfectly good producer.
In Go:

```go
import "github.com/emusoi/bango"

p := bango.Panel{
    Version: bango.Version, ID: "items", Title: "items",
    Sections: []bango.Section{{ID: "all", Rows: rows}},
}
```

The package is types, validation and a relative-time helper. It has no
dependencies and does not render.

## Writing a renderer

Read [SPEC.md](SPEC.md), then make `fixtures/` reproduce. The fixtures are the
conformance suite and the specification's tests.

## What it is not

Not a widget toolkit, not a layout engine, not a forms library, not a TUI
framework. It describes a titled set of sections containing rows with actions,
and refuses everything else.

MIT.

## Panels of panels

An action whose command prints a panel pushes it; `q` pops back. `examples/launcher`
is a repository menu in twenty-five lines of shell:

```sh
$ bango -- examples/launcher
this repository
   branches
   working tree
```

That is one mechanism, not a second kind of action: run a command, and if what
comes back is a panel, render it.

## License

MIT. See `LICENSE`.
