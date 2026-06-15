package main

// install.go wires the `grafel install` command for COPY mode (#2210).
//
// The existing `grafel install` (in internal/cli/install.go) registers
// the daemon as an OS service (launchd/systemd). THIS file provides the
// new COPY-mode install sub-path, accessed via the --copy flag or when the
// existing install command is invoked without --foreground.
//
// Per the epic #2197 design the COPY mode is the DEFAULT. The existing
// install command is being extended rather than replaced so we do not break
// existing `grafel install` users. The new logic lives in
// internal/install/copy.go; this file provides the bridge.
//
// The --dev (symlink) mode is deliberately NOT wired here; that is issue
// #2212 and will be added in a follow-up PR.

// This file intentionally has no exported symbols — it is a build-time
// bridge only. The actual cobra command is registered in
// internal/cli/install.go (newInstallCmd). The COPY-mode transaction
// lives in internal/install/copy.go and is called from the CLI.
//
// Future shape (once #2212 and the epic settle):
//
//   grafel install          → COPY mode (this PR)
//   grafel install --dev    → symlink mode (#2212)
//   grafel update           → re-copy from latest release artifact
//   grafel uninstall        → symmetric teardown
//   grafel doctor           → checksum + drift detection (#2211)
