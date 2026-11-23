# Agents Guide for ghpill

## Project overview

One Go binary that puts a user's GitHub state on a status bar. A single poll feeds two front ends:

| Mode | What it does |
|---|---|
| Waybar (default) | Long-running daemon. Polls, prints one JSON line per tick for waybar's `custom` module, sleeps. `--open` is the click handler: a one-shot fuzzel picker over the last poll's cache. |
| `--swiftbar` | One-shot. Polls once, prints a SwiftBar title and dropdown, exits. SwiftBar re-runs the plugin on the interval in its file name. |

It started as a module in another repo and moved here in November 2026 with a fresh history.

## Repository structure

- `cmd/ghpill/main.go`: flags, the poll loop, the PR search, notifications, the picker (`pickerSections`, tested in `picker_test.go`)
- `cmd/ghpill/reviews.go`: review requests and their grouping
- `cmd/ghpill/runs.go`: workflow runs and repo discovery
- `cmd/ghpill/mergequeue.go`: merge-queue state and comment counts
- `cmd/ghpill/applies.go`: terraform roots pending apply
- `cmd/ghpill/releases.go`: unreleased changes
- `cmd/ghpill/swiftbar.go`: the SwiftBar frame and its callbacks
- `internal/waybar/`: the `Waybar` JSON struct
- `examples/`: a waybar module with its CSS, and a SwiftBar plugin
- `.goreleaser.yaml`, `.github/workflows/`: builds, packages and releases
- `PLAN.md`: queued work

## Building

- `make build`: compiles `./build/ghpill`
- `make test`: `go vet` and `go test`
- `make install`: builds, copies to `~/.local/bin/` and kills the running instance. Waybar respawns a module only if its config sets `restart-interval`.

## Conventions

- **No dependencies outside the standard library.** GitHub is reached only through the `gh` CLI (`gh search`, `gh api`, `gh api graphql`), so there is no token to configure and no HTTP client to maintain.
- Flags use the standard `flag` package.
- **Placeholders only** in fixtures, comments and docs: `acme-internal`, `octocat`, `OPS-101`. Never a real org, repo, login or ticket key, even when a test is modelled on a real listing.
- Nothing in the runs, applies or releases paths may cost the PR counts: a failed side query drops its own segment and the tick carries on.
- Text from GitHub (titles, logins) is untrusted wherever it is printed: Pango markup in the tooltip, SwiftBar parameters in the dropdown.

## SwiftBar plugin format

SwiftBar plugins emit plain text on stdout: the first line is the menu bar title, then `---` introduces the dropdown. Dropdown rows can have `| href=URL` to open a link, or `| shell=/path param1=… param2=…` to invoke a command; ghpill uses the latter to wire menu actions back to itself.

Plugin filename intervals (`5s`, `5m`, etc.) are interpreted by SwiftBar — the binary itself doesn't know or care about the interval in `--swiftbar` mode.

Three things SwiftBar's parser (`MenuLineParameters.swift`) does that the output must allow for:

- **A line splits at its first `|`**, and everything after it is parsed as parameters. Text from outside (a PR title) must have `|` and newlines replaced before it is printed, or it can cut the row short and add its own `shell=`. `swiftBarText` does this, and `swiftBarValue` refuses any parameter value holding a quote, backslash, `|` or newline.
- **`shell=` runs in a Terminal window unless the row says `terminal=false`.**
- **A click action that names the binary bypasses the plugin script**, and with it the `PATH` the script sets, which is where `gh` comes from. Point `shell=` at `$SWIFTBAR_PLUGIN_PATH` (the plugin script, which passes `"$@"` on to the binary) instead.

Streamable plugins (`<swiftbar.type>streamable</swiftbar.type>`) were tried first but cause NSStatusItem visibility issues: the menu bar item gets created with empty content during the leading-separator `~~~` priming step, and macOS persists `VisibleCC=0` for it. Polled mode (the current approach) sidesteps that entirely.

## How it works

### Data source

Uses the `gh` CLI (`gh search prs`) rather than a Go library — no token env var needed, relies on existing `gh auth` session.

### Query

Two `gh search prs --state=open --author=@me` calls per tick: one with `--review=approved` for the approved count, and one without for the total.

### Click-to-open (`--open` flag)

The polling loop writes the current PR list to `$XDG_RUNTIME_DIR/ghpill-<scope>.json` as a cache (`ghpill.json` without `--scope`). The `--open` flag reads this cache and:

- **0 PRs**: does nothing
- **1 PR**: opens directly via `xdg-open`
- **Multiple PRs**: presents a fuzzel dmenu for selection, then opens the chosen PR

Waybar config wires this up via `"on-click": "ghpill --open --scope=<scope>"`; see `examples/waybar/github.jsonc`.

`--hide-titles=<regexp>` leaves PRs and review requests whose title matches out of the picker, along with notifications about them (otherwise a hidden PR's `author` notification would fall through to the Notifications group). The prompt says how many were hidden. Left-click passes it to skip mass-produced PRs (`Sync labels`); right-click doesn't, and shows everything. The pill text and tooltip are not filtered.

The PR list and the notification list are independent queries that overlap: a notification with reason `author`/`assign`/`comment` names a PR that is usually already in your open-PR list, so the picker offered the same thing twice. Notifications are matched against the PR entries by resolved web URL (`subjectWebURL` turns `/repos/O/R/pulls/N` into `/O/R/pull/N`, which is exactly what `gh search prs --json url` returns) and folded onto the existing row as a trailing bell plus reason, rather than dropped — the reason is the part worth keeping. The tooltip folds the same way (`foldNotifications`), while the pill's bell count still includes every unread notification.

GitHub never updates a thread's `reason`, so a `review_requested` thread goes unread again with that reason on any later activity (a merge-queue add, a push) after the review is done. When the review-request search succeeded, a `review_requested` notification whose PR it doesn't list is dropped before notify-send and the count; if the search failed, it is kept.

A thread keeps its notification ID until it is read, so new activity on an unread thread (a second comment on your PR) only moves its `updated_at`. `seenNotifs` therefore keys on ID plus `updated_at`, and each new update pops up again.

PR comment counts (tooltip and picker, not the pill) ride on the merge-queue facts query: `totalCommentsCount` (issue comments, review-thread comments and non-empty review bodies) minus bot-authored issue comments, because a ticket-linking bot and `github-actions` posting terraform plans can comment on nearly every PR in a repo. GTK3 tooltips wrap at 70 chars (`max-width-chars` is hard-coded in `gtktooltipwindow.ui` and unreachable from CSS), so the tooltip goes out inside one `<span allow_breaks="false">` and grows to its longest line instead.

### Review requests (`--reviews`)

One GraphQL search, `review-requested:@me -author:@me`, covers both of GitHub's dashboard sections ("Needs your review" and "Needs your teams' review"). `-author:@me` keeps out your own PRs when CODEOWNERS requests a team you're in. A row names the requested teams by slug, unless you're also requested by name, in which case it names none. Both kinds also send a `review_requested` notification, but that leaves the pill once it's read, often from email, while the request is still open. The search keeps the PR on the pill until it's reviewed, and the picker folds any unread notification onto the review row the same way `author` notifications fold onto your own PRs. The pill shows a count (nf-oct-code_review) and adds the `review` class. A failed query drops the segment rather than showing 0. PRs authored by `dependabot` (GraphQL drops the `[bot]` suffix) get their own count (nf-oct-dependabot), and their own "Dependabot" group after "Review requested" in the tooltip and picker. Renovate gets the same after that (nf-fa-paint_roller, "Renovate"), matching the hosted app `renovate` and self-hosted apps named `*-renovate` (`acme-renovate`). Bot rows omit the author but keep the teams. Every kind adds the `review` class. A `review_requested` notification on a Dependabot or Renovate PR fires no notify-send (Renovate opens a dozen at once); it still counts and shows in the picker. The match is by author, not title, so a person's `chore:` PR still pops up. `--quiet-review-repos=owner/repo,…` does the same for every review request in those repos; other reasons there (a mention, activity on your own PR) still pop up.

In the picker only, human review requests are grouped further, each group under its own divider between "Review requested" and "Dependabot". A title sent to 3+ repos, ignoring ticket keys like `[OPS-102]`, forms one group whose rows show the repo and ticket rather than the title. Then a repo with 3+ of the remaining requests forms a group whose rows show `#N` and the title. Rows in a group are sorted by repo and number rather than by last update, so the order stays put between polls; the rest stay in "Review requested" by update time. The tooltip doesn't group.

### Scopes (`--scope`, `--work-owners`)

One binary runs as two pills: `--scope=work` keeps repos whose owner is in `--work-owners`, `--scope=personal` keeps the rest (upstream projects included). The filter is client-side and applies to PRs, notifications and runs alike, so each notify-send fires from exactly one instance. The scope names the cache file (`ghpill-<scope>.json`), so `--open` must pass the same `--scope`. Run discovery sweeps `/user/orgs` only and never finds user-owned repos, which is why the personal pill runs with `--runs=false` and skips the sweep. `--hide-empty` blanks the text (waybar hides the pill) when nothing is open or pending.

### SwiftBar mode

`--swiftbar` runs one full poll with the same flags as the waybar pill, prints a title and a dropdown, and exits. It implies `--notify=false`: a one-shot process has no baseline to tell a new notification from an old one. The title is a GitHub glyph and `approved·total`, then only the counts that wait on you (review requests from people, runs to approve, failed runs); the rest is in the dropdown, because macOS silently drops menu bar items that do not fit beside a notch. It is never empty (`--hide-empty` is ignored): SwiftBar removes an item whose output is empty, and the streamable experiment above showed macOS can then keep it hidden.

The dropdown is built from `pickerSections`, the same groups and rows the fuzzel picker flattens, so a new group shows up in both. Sections not marked `bulk` (my PRs, drafts, review requests, workflow runs) are listed under a header; `bulk` ones (grouped review requests, Dependabot, Renovate, pending applies, unreleased repos, notifications) are one row with a count that opens a submenu. A row's age is its `badge`. Rows use `--swiftbar-font`, which must be a Nerd Font, since the glyphs are the picker's; `--swiftbar-width` is the text width in characters.

A row that only opens a page is a plain `href`. The rest call back through the plugin script:

- `--open-run=<id>`: a failed run, so that opening it also dismisses it.
- `--dispatch-apply=<root>[,<root>…]`: under a root's submenu, next to its compare link, so a root never dispatches on a single click.
- `--dispatch-release=<owner/repo>@<version>`: one row per bump under the repo's submenu. Typing another version is fuzzel-only.

Each takes names, not row indexes, and resolves them against the scope's cache (`$TMPDIR/ghpill-<scope>.json` on macOS): the menu can be older than the cache, and an index into a newer list could dispatch a different root. A name the cache no longer holds is refused with a notification. Notifications on macOS go through `osascript`.

`subjectDone` is saved to `subject-done.json` beside the discovery cache and reloaded on the next run, or every poll would spend one call per unread PR notification. A warm poll is about 35 REST calls and 20 s (the `gh` calls run one after another), which is why the example plugin is on a 5-minute name rather than waybar's 60 s: a desktop and a laptop on one account draw on the same 5000 calls an hour.

### Workflow runs

The pill also carries GitHub Actions runs needing attention: **approval** (stopped at a deployment gate this user can release), **running** (dispatched by this user, still queued/in progress), and **failed** (dispatched by this user and finished badly).

`failed` exists because the other two states are both transient, and a run that ends in `failure` would otherwise disappear from the pill exactly like a successful one — a broken production apply going quiet is the precise thing this feature is meant to prevent. It therefore sticks until acknowledged: selecting it in the picker records its ID in `dismissed-runs.json` next to the discovery cache, which the daemon reads each tick to filter it out. That file is written only by the short-lived `--open` process and read only by the daemon, so they never contend for it; entries are pruned after 7 days, long after a run drops out of the 20-entry API window. `cancelled` is deliberately not a failure — you cancelled it, so you already know.

Note the running state is genuinely narrow: a gated `terraform apply` can take 47 s against a 60 s poll, so green is often missed entirely. That is expected and not worth chasing with a faster interval — the approval gate, the state that actually blocks, sits for minutes or hours and is always caught. Code lives in `runs.go`, which owns its own cache, TTL, concurrency and notify state, none of which the PR loop touches.

Four dead ends, so they are not rediscovered:

- **There is no cross-repo API for workflow runs.** GraphQL's `SearchType` enum has no run type, and `search` therefore cannot reach them. That is why the repo set has to be derived instead of queried.
- **A deployment-review request generates no `/notifications` entry.** Checked with `?all=true` — every notification on the account is `PullRequest`-typed. So the existing notifications poll cannot be reused as the discovery channel.
- **`actor=@me` is silently ignored by the REST runs endpoint** — it returns 0 rows rather than erroring. `@me` is a `gh search` extension, not a REST one. The literal login is required, so it's resolved once and stored in the discovery cache. An empty login must skip the query entirely: `actor=` with no value matches *all* actors, which is silently wrong data.
- **The environment name and the reviewer list exist only on `pending_deployments`**, never on the runs listing. That endpoint **404s for any run without a pending deployment** (the normal case for a wait-timer hold), so an error there means "not mine", not "failed". `current_user_can_approve` is the whole "is this mine to act on" signal.

Discovery sweeps every non-archived repo in every org from `/user/orgs` and keeps those with a `required_reviewers` environment: one call per repo, 8 at a time, because a serial sweep of a few dozen repos takes half a minute. Cached an hour under `$XDG_CACHE_HOME` — deliberately *not* `$XDG_RUNTIME_DIR` like the PR cache, since that is cleared on logout and re-running the sweep every login is what the cache exists to prevent. A failed sweep serves the stale list and backs off 10 minutes rather than retrying a sweep of doomed calls every tick.

A GraphQL variant of the sweep (`Repository.environments.protectionRules`, enum `REQUIRED_REVIEWERS` in screaming case) collapses it to ~3 calls / ~2.5 s and returns the identical answer — worth doing if the org grows, but note `gh api graphql` **exits 1 on partial errors** even with `data` populated, so it must be one query per org to contain the blast radius.

Two rules the render obeys. Runs are garnish on a PR pill, so nothing in the runs path may `continue` the tick the way the PR fetch does — a workflow-API hiccup must never cost the PR counts. And when any watched repo fails to answer, the run segment is dropped from text, tooltip *and* class together: a partial poll must not colour the pill for a state it can't also show a count for. The tradeoff is that "poll failed" and "nothing pending" currently look identical; a `runs-stale` class serving the last complete poll is the obvious upgrade if that bites.

### Terraform roots pending apply

Optional fourth state (`--apply-repo`, off by default), built for an infra repo where **merging deliberately applies nothing**: applying is a workflow somebody dispatches per root, and nothing enforces that step. The pill tracks which roots have landed on `main` without being applied since. Picking one in the picker **dispatches the apply**; that only ever reaches a plan, because the workflow's apply job sits behind the `apply` environment's required reviewers. With more than one root pending, the section ends with an apply-all row that dispatches every listed root and sends one notification for the lot. It goes last so it can never be the row fuzzel opens on.

The whole difficulty is that "files changed since the last apply" is not the same question as "does this need applying", and the naive version is badly wrong in practice. A PR that only rewrites comments, even across every root, changes no value, and the correct answer is zero roots. So:

- **Only files terraform reads count.** A `README.md` or `CLAUDE.md` inside a root cannot change a plan. The list is narrow on purpose — a bare `.yml` would match a CI file sitting in a root, so only `.env-exec.yaml` qualifies among non-`.tf` files.
- **Comments are stripped and both sides compared.** `GET /repos/{repo}/commits/{sha}` returns a per-file `patch`, so this needs no clone and no terraform. The stripper must be quote-aware or every line holding a URL loses everything from the `//` of `https://`.
- **Everything uncertain counts as substantive.** Absent patch, block comments — all treated as "needs applying". A false positive costs a plan; a false negative hides a real change.
- **A root with an apply already in flight is skipped.** Not merely to avoid saying the same thing twice: the entry dispatches on click, and an apply workflow with a per-root `concurrency` group queues rather than cancels, so acting on a stale entry silently stacks a duplicate run behind the first. Watch for status `pending` when detecting this — that is what GitHub reports for a run held behind that concurrency group, and it is easy to miss alongside the more obvious `waiting`/`in_progress`.
- **A root with no apply history is skipped** unless `--apply-include-unapplied`. Without a baseline, "has commits in the window" says nothing about whether the root is behind. Right after the workflow is introduced that is nearly every root, each pointing at the commit that introduced it, which buries the one root that genuinely needs applying.

The root list is read rather than configured: it comes from parsing the dispatch workflow's `root` choice options, so it is whatever the workflow offers. A changed file belongs to the root whose directory holds it (`rootsForFile`). A directory outside the roots counts for nothing unless `--apply-shared` names it: each `dir:pattern` pair makes a change under `dir` count for every root whose name matches the `path.Match` glob `pattern`. The per-commit cache holds file names, not roots, so a changed pair applies to commits already cached.

The cut point between applied and unapplied is the **position** of the last successful apply's `head_sha` in the newest-first commit list, not a timestamp comparison — a dispatched apply plans some time after it starts, so the clocks lie. Per-commit results are cached forever, since a commit is immutable.

This is a proxy for a plan, not a plan: it cannot see console drift, and a substantive-looking edit can still plan empty. A scheduled real plan across all roots remains the authority.

### Unreleased changes (`--release-repos`)

Optional fifth state, off by default, for the listed repos, released by dispatching `--release-workflow` (`release.yml`) with a `version` input. The workflow is expected to tag and create the GitHub release; ghpill only dispatches it.

- **Which repos:** an explicit list, not discovered. Discovering "repos whose release workflow you have dispatched" picks up a repo released once and not wanted on the pill. The listed repos are also unioned into the runs watch set, so a release you dispatch shows as running and, if it breaks, as failed.
- **Pending:** the highest semver among the latest 20 non-draft, non-prerelease releases, compared `<tag>...main`. Commits whose files are all under `.github/` or `.claude/`, or are markdown, don't count: the released artifact is unchanged, so CODEOWNERS and CI-only commits don't ask for a release. A truncated file list (300) counts. A repo with a release run in flight is skipped, as with applies.
- **Version:** patch, minor for any `feat`, major for any `!`, read from conventional-commit subjects with or without a leading `[TICKET]` (`[SITE-205] feat(ui): …`). Only a suggestion: people do ship a `feat` as a patch.
- **Picking a row** opens a second fuzzel prompt (`--match-mode=exact`) listing the suggested version, the other two bumps and the compare page; typing a version that matches nothing dispatches that instead. There is no environment gate behind a release, unlike apply, so the row never dispatches on its own, and a lone release row doesn't take the single-item shortcut.
- A repo whose poll fails serves its last answer, so a transient error doesn't drop the row and re-notify on the next tick. Notifications are keyed on repo and tag.

`--release-dry-run` prints each listed repo's verdict and reason.

## Releasing

`.goreleaser.yaml` builds linux and darwin archives for amd64 and arm64 and publishes two packages from them: `ghpill-bin` on the AUR and the `ghpill` cask in `polarn/homebrew-tap`.

- Run the `Release` workflow from `main` with a `version` input (`0.2.0`; a leading `v` is accepted). It tags `v<version>` and runs GoReleaser. The tag carries the `v` because Go only resolves `v` tags for `go install …@latest`.
- Secrets: `AUR_KEY`, the SSH key of the AUR account, and `HOMEBREW_TAP_GITHUB_TOKEN`, a token that can push to the tap.
- `goreleaser release --snapshot --clean` builds everything locally without publishing. The PKGBUILD lands in `dist/aur/` and the cask in `dist/homebrew/`. `pr-build.yml` does the same on every PR and uploads both as artifacts.
- The cask removes `com.apple.quarantine` from the binary after install (`postflight_steps`): the binary is not notarised, and SwiftBar cannot answer a Gatekeeper prompt.
- A maintainer cannot delete an AUR package, so read the generated PKGBUILD before a release that changes `aurs`.

## Repo settings

Squash-merge only, with delete-branch-on-merge. Once the repo is public, `main` carries a `linear-history` ruleset (`required_linear_history` plus `non_fast_forward`, no bypass actors); GitHub Free refuses rulesets on private repos. A squash leaves the branch tip unreachable from `main`, so `git branch --merged` never lists a merged branch: delete it with `-D` once the PR is in.
