# golangci-lint cache contaminates pre-push across sibling worktrees

## Symptom

`git push` fails with a flood of `golangci-lint` issues referencing files in
**other** worktrees you don't have checked out right now. Example:

```
WARN [runner/source_code] Failed to get line 42 for file
/home/cybot/projects/ofelia/harden-preset-cache/core/context_propagation_test.go:
... no such file or directory

../harden-preset-cache/core/common.go:51:2: found a struct that contains a
context.Context field (containedctx)
../harden-preset-cache/core/runjob.go:172:31: Non-inherited new context, use
function like `context.WithXXX` instead (contextcheck)
... 23 issues ...

error: failed to push some refs to 'github.com:netresearch/ofelia.git'
```

The lefthook `pre-push` hook runs `golangci-lint run --timeout=3m`. Pushing
your clean branch is blocked by lint findings from worktrees that:

- are siblings of the one you're pushing (e.g. `../harden-preset-cache/`),
- were removed since the cache was written, **or**
- are simply not the worktree you currently care about.

## Cause

`golangci-lint` stores results in a shared cache directory
(`$GOLANGCI_LINT_CACHE`, defaults to `$XDG_CACHE_HOME/golangci-lint` →
`~/.cache/golangci-lint`) keyed by file path + content hash. When the cache
was warm from a previous run in a sibling worktree, those paths get replayed
even though the current `golangci-lint run` only walks the current
worktree's packages. Stale entries become "issues" in the report.

## Fix

Clear the cache before pushing:

```sh
golangci-lint cache clean
git push
```

Faster check — only run if a sibling worktree was added, removed, or
checked-out-to-a-different-branch since your last successful push from this
worktree. Day-to-day on a single worktree this isn't an issue.

## Long-term options (not yet adopted)

- Pin the cache per worktree:
  `export GOLANGCI_LINT_CACHE="$(git rev-parse --git-common-dir)/golangci-lint-cache/$(git rev-parse --abbrev-ref HEAD)"`
- Have `lefthook.yml` `pre-push.golangci-lint` step run `cache clean` first
  (trades cache reuse for correctness).
- Use `--new-from-rev=origin/main` so only changes added in your branch are
  evaluated; out-of-tree cache entries can't surface as findings.

## History

- 2026-05-15 — encountered during PR #671 push. Two consecutive pushes
  failed (~75 s of hook time wasted) before the cache was cleared. Recorded
  as `docs/feedback/` after `/retro` session sweep flagged it.
