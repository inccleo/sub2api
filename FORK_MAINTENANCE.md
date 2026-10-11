# Maintaining this fork

## Owner requirement: preserve customizations (2026-10-11)

Every upgrade must preserve existing TopAPI features, configuration, and styling.
An upgrade request does **not** authorize replacing or removing customizations.
If a customization cannot be preserved, prepare the concrete before/after changes
and their impact, then ask the owner for explicit approval before merging or
deploying that replacement. Continue independent upgrade work while awaiting the
decision; do not treat silence or earlier upgrade approval as consent.

Use the current custom release as the comparison baseline. Review upstream changes
against the fork, even when Git reports no conflicts. Do not take entire upstream
files over customized versions, or delete/relax regression tests to bypass failures.
Keep `AGENTS.md` and these requirements in every future integration branch.

Protected recharge behavior and presentation include:

- The 50 / 100 / 500 / 1000 package cards, their large card layout, names,
  descriptions, recommendation labels, gifts, and credited totals.
- The enterprise cooperation card, “联系商务” button, and QR-code dialog.
- The existing `PAYMENT_ENTERPRISE_QR_CODE_URL` and contact configuration;
  upgrades must not reset them or substitute a different QR code.
- The fixed gifts and explicit-tier precedence documented below.

Before release, run the existing recharge regression checks and compare the page
against the baseline, including the contact dialog and loaded QR image. Record
the result in the upgrade PR. A passing build or `/health` check is insufficient.
If anything else in the custom diff is unclear, preserve it pending investigation
or explicit approval; this list does not authorize removal of other customizations.

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

The `Custom release` workflow publishes only tags formatted as `vMAJOR.MINOR.PATCH.CUSTOM`, such as `v0.1.162.1`. It builds the Linux `amd64` archive and `checksums.txt`; full unit tests run on branch `CI`, and focused recharge regressions also gate every custom release, and an unchanged frontend dist is restored from cache instead of rebuilt. The inherited `Release` workflow explicitly excludes four-part tags so both workflows cannot write the same GitHub Release; do not dispatch the inherited workflow manually for production.

Production sets `UPDATE_REPOSITORY=inccleo/sub2api`. `backend/internal/service/update_service.go` validates this setting before it calls the GitHub API, so the administrator update flow resolves this fork rather than the upstream project. The `v2.10.0` production release line is based on upstream `v2.10.0` and published from `custom-main-ranxi-v2.10.0`; see the private operations runbook for the currently deployed custom patch. Do not use the inherited upstream `Release` workflow for production artifacts.

The private operations runbook, deployment procedure, database backup requirement, and rollback steps live outside this source repository in `topapi/docs/deploy/sub2api-custom-fork-update-runbook.md`.

## Recharge packages: mandatory upgrade gate

When `RECHARGE_BONUS_TIERS` is empty, preserve the TopAPI fixed packages:
50 → 50, 100 → 120, 500 → 650, 1000 → 1400 (before the configured balance multiplier).
`checkout-info`, package cards, the checkout summary, and `CreateOrder` must agree.
Explicit upstream tiers replace these gifts, never stack with them. Non-package API
amounts do not receive fixed gifts; subscription pricing remains separate.
Persist the free USD portion in `bonus_amount` so affiliate rebates exclude gifts.

Before every upstream upgrade, run `make test-frontend` (including AmountInput and
PaymentView), plus `go test -tags=unit ./internal/service -run TestCreateOrderPreservesRechargePackages -count=1`
from `backend`. The latter creates real database orders and verifies the gateway amount.
A failing customization test blocks release; do not dismiss it as stale without
confirming the business rule. Verify package cards and totals on the deployed purchase page.
