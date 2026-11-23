# Plan

Work queued for this repo. Each item says what done looks like. Remove an item once it lands; its PR keeps the reasoning.

## 1. A config file

The pill, its click handlers and the SwiftBar plugin each repeat `--scope`, `--work-owners` and the repo lists, and they must agree or `--open` reads the wrong cache.

- Done: one file holds the scopes, owners and repo lists, flags still override it, and the examples pass only `--scope`.

## 2. Apply tracking without the built-in layout

`applies.go` assumes one infra repo's shape:

- Runs are matched by the title `terraform apply <root> (@<login>)`.
- The branch is `main`, here and in `releases.go`.
- Labels say "terraform".

Done: the default branch is read from the repo, and the README documents the run-title contract or it is a flag.

## 3. A picker other than fuzzel

`--open` and the release prompt call `fuzzel --dmenu` with fuzzel-only flags (`--index`, `--select-index`, `--match-mode`).

- Done: a flag picks the program, and the picker works with at least one other dmenu-compatible launcher.

## 4. Notifications on macOS

`--swiftbar` implies `--notify=false`, because a one-shot process has no baseline to tell a new GitHub notification from an old one. Dispatch results already go through `osascript`.

- Done: the seen set is kept between runs, and a new notification pops up once.

## 5. A demo mode

The README has no screenshots, because a real poll shows real pull requests.

- Done: `--demo` renders the pill, the picker and the dropdown from built-in sample data, and the README shows both front ends.
