# ghpill

Your GitHub work as one pill on the status bar: a [Waybar](https://github.com/Alexays/Waybar) module with a [fuzzel](https://codeberg.org/dnkl/fuzzel) picker on Linux, and a [SwiftBar](https://swiftbar.app) menu bar item with a dropdown on macOS.

The pill reads `approved·total` for your open pull requests, followed by a glyph and a count for each thing that is waiting:

- pull requests whose review is requested from you or one of your teams, with Dependabot and Renovate counted apart
- unread notifications
- your pull requests sitting in a merge queue, or refused by one
- GitHub Actions runs stopped at a deployment gate you can approve, runs you dispatched that are still going, and ones that failed
- optionally, Terraform roots merged to `main` but not applied since (`--apply-repo`)
- optionally, repositories with changes on `main` that are not released (`--release-repos`)

A click lists all of it. Picking a row opens the page; a pending apply or release row dispatches its workflow instead.

## Requirements

- The [GitHub CLI](https://cli.github.com), logged in with `gh auth login`. ghpill has no token of its own: every call goes through `gh`.
- A [Nerd Font](https://www.nerdfonts.com) in the bar, for the glyphs.
- Linux: Waybar, `fuzzel`, `notify-send` and `xdg-open`.
- macOS: SwiftBar.

## Install

Arch Linux, from the AUR:

```
yay -S ghpill-bin
```

macOS, with Homebrew:

```
brew install --cask polarn/tap/ghpill
```

From source, with Go 1.25 or later:

```
go install github.com/polarn/ghpill/cmd/ghpill@latest
```

## Waybar

ghpill runs as a daemon that prints one JSON line per poll. A minimal module:

```jsonc
"custom/github": {
  "format": "{text}",
  "return-type": "json",
  "restart-interval": 1,
  "exec": "ghpill --interval 60",
  "on-click": "ghpill --open"
}
```

[`examples/waybar/github.jsonc`](examples/waybar/github.jsonc) has a work pill and a personal pill, and [`examples/waybar/style.css`](examples/waybar/style.css) the colours.

`class` is an array: `none` or `found` (an approved pull request exists), plus any of `queued`, `running`, `pending`, `review`, `approval`, `dequeued` and `failed`. GTK breaks ties between equally specific rules by document order, so the order of the rules in `style.css` decides which colour wins. Keep `failed` last.

## SwiftBar

Copy [`examples/swiftbar/github-pr.5m.sh`](examples/swiftbar/github-pr.5m.sh) into SwiftBar's plugin folder and make it executable. `--swiftbar` polls once, prints the title and the dropdown, and exits. SwiftBar runs it again on the interval in the file name. A poll is about 35 API calls and takes about 20 seconds, so 5 minutes is a sensible interval.

## Flags

| Flag | Default | What it does |
|---|---|---|
| `--interval` | `120` | Seconds between polls in Waybar mode |
| `--scope`, `--work-owners` | all repos | `--scope=work` keeps repos owned by `--work-owners`, `--scope=personal` keeps the rest. Run one pill of each |
| `--open` | | Open the picker and exit. Pass the same `--scope` as the pill |
| `--hide-titles` | | With `--open`, leave out pull requests whose title matches this regexp |
| `--hide-empty` | `false` | Print nothing, which hides the pill, when nothing is open or pending |
| `--notify`, `--notify-reasons` | `true` | Desktop notifications for new GitHub notifications, and which reasons count |
| `--quiet-review-repos` | | Repos whose review requests count but send no desktop notification |
| `--reviews`, `--runs`, `--merge-queue` | `true` | Turn a group off |
| `--watch-runs` | | Extra repos to poll for runs, on top of the discovered ones |
| `--failed-ttl` | `72h` | How long a failed run stays on the pill |
| `--apply-repo`, `--apply-workflow` | off, `apply.yml` | Track Terraform roots pending apply in this repo, and the workflow that applies one |
| `--apply-shared` | | `dir:pattern` pairs for directories outside the roots: a change under `dir` counts for every root matching the glob `pattern` |
| `--release-repos`, `--release-workflow` | off, `release.yml` | Track unreleased changes in these repos, and the workflow that releases one |
| `--swiftbar`, `--swiftbar-font`, `--swiftbar-width` | | SwiftBar output, its font and its row width in characters |
| `--version` | | Print the version and exit |

`ghpill --help` lists the rest, including the `--*-dry-run` flags that print what a group found and why.

## What apply and release tracking expect

Both are built around workflows started by hand with `workflow_dispatch`, and both compare against a branch named `main`:

- `--apply-repo`: the apply workflow has a `root` choice input listing the Terraform roots, and titles its runs `terraform apply <root> (@<login>)`. Each root is a directory of that name. Changes that only touch comments, or files Terraform does not read, do not count. A directory several roots read from, such as local modules, belongs to no root, so name it: `--apply-shared='modules/network:app/*'` counts a change under `modules/network/` for every `app/*` root.
- `--release-repos`: the release workflow takes a `version` input and creates a GitHub release. The suggested version comes from the conventional-commit subjects since the last release.

## Files

- `$XDG_RUNTIME_DIR/ghpill-<scope>.json` (`$TMPDIR` on macOS): the last poll, read by `--open` and the dropdown's actions.
- `~/.cache/ghpill/`: discovered repos, dismissed runs and per-commit results.

## License

[Apache-2.0](LICENSE)
