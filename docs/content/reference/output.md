---
title: "Output formats"
description: "The output contract every command shares: formats, fields, and templates."
weight: 30
---

Every list command in the fleet renders through one formatter, so the same flags
work everywhere. Wire your commands through it as you add them, and this page
describes what users get. Pick a format with `-o`, or let ssllabs choose:
a table when writing to a terminal, JSONL when piped.

## Formats

```bash
ssllabs <command> -o table   # aligned columns for reading
ssllabs <command> -o jsonl   # one JSON object per line, for piping
ssllabs <command> -o json    # a single JSON array
ssllabs <command> -o csv     # spreadsheet friendly
ssllabs <command> -o tsv     # tab-separated
ssllabs <command> -o url     # just the URL column
ssllabs <command> -o raw     # the underlying bytes, unformatted
```

| Format | Best for |
|---|---|
| `table` | Reading on a terminal |
| `jsonl` | Piping into another tool, one object at a time |
| `json` | Loading a whole result as an array |
| `csv` / `tsv` | Spreadsheets and quick column math |
| `url` | Feeding URLs into other commands |
| `raw` | The unformatted bytes (response bodies, file contents) |

## Narrowing columns

Keep only the fields you want:

```bash
ssllabs <command> --fields id,title,url
```

`--no-header` drops the header row in `table` and `csv` output, which helps when
a downstream tool expects bare rows.

## Templating rows

For full control over each line, apply a Go text/template. Fields are the JSON
keys, capitalised:

```bash
ssllabs <command> --template '{{.URL}} {{.Title}}'
```

## Why auto-detection helps

Because the default adapts to the destination, the same command reads well by
hand and parses cleanly in a pipe:

```bash
ssllabs <command>            # a table, because this is a terminal
ssllabs <command> | wc -l    # JSONL, because this is a pipe
```

You only reach for `-o` when you want something other than that default.
