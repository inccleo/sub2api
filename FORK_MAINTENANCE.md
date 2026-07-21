# Maintaining this fork

This repository tracks [`Wei-Shaw/sub2api`](https://github.com/Wei-Shaw/sub2api) while carrying local TopAPI changes.

## Branches

- `main` is a clean mirror of upstream `main`. Do not add local commits to it.
- `custom-main` is the integration and production-release branch.
- Use `feature/*` for local work and `sync/*` for each upstream merge.

Sync upstream through a PR:

```bash
git fetch upstream --prune --tags
git switch main
git merge --ff-only upstream/main
git push origin main

git switch custom-main
git switch -c sync/upstream-vX.Y.Z
git merge --no-ff main
git push -u origin sync/upstream-vX.Y.Z
```

## Third-party PRs

Cherry-pick only the reviewed commits and preserve their origin:

```bash
git fetch upstream refs/pull/123/head:refs/remotes/upstream/pr-123
git switch -c feature/pr-123 custom-main
git cherry-pick -x <commit-sha>
```

## Releases and in-app updates

The `Custom release` workflow publishes only tags formatted as `vMAJOR.MINOR.PATCH.CUSTOM`, such as `v0.1.162.1`. Its Linux `amd64` archive and `checksums.txt` match the application's updater contract. The inherited `Release` workflow explicitly excludes four-part tags so both workflows cannot write the same GitHub Release; do not dispatch the inherited workflow manually for production.

Production sets `UPDATE_REPOSITORY=inccleo/sub2api`. `backend/internal/service/update_service.go` validates this setting before it calls the GitHub API, so the administrator update flow resolves this fork rather than the upstream project. Do not use the inherited upstream `Release` workflow for production artifacts.

The private operations runbook, deployment procedure, database backup requirement, and rollback steps live outside this source repository in `topapi/docs/sub2api-custom-fork-update-runbook.md`.
