# TopAPI fork: upgrade requirements

Before upgrading or syncing upstream, read `FORK_MAINTENANCE.md` and preserve all
existing TopAPI customizations, including behavior, configuration, and UI styling.

The owner's explicit instruction (2026-10-11): upgrades must not overwrite their
customizations. If replacement or removal is necessary, first prepare a concrete
before/after comparison and explain the impact, then obtain the owner's explicit
approval for those changes before merging or deploying them. A request to upgrade
or approval of a previous upgrade is not approval to discard customizations.

In particular, preserve recharge gifts, package card styling and labels, the
enterprise contact card, and the business QR-code dialog and configuration.
Do not resolve conflicts by taking upstream files wholesale or remove/relax
customization regression tests to make an upgrade pass. Failed customization
checks block release until fixed or the specific behavior change is approved.

Keep this file and the customization requirements in `FORK_MAINTENANCE.md` in
version control through every upstream sync. Verify the UI as well as the amounts;
a successful build or health endpoint alone is insufficient.
