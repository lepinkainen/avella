# Full Swift Rewrite: Analysis

Decision (2026-03-28): **Sticking with Go daemon + SwiftUI tray app.** The config UI complexity is the primary blocker.

## Pros of a full Swift rewrite

- **Single language/toolchain** -- no Go toolchain needed, one build system, simpler contributor onboarding.
- **Native macOS integration** -- direct access to FSEvents, `NSWorkspace`, `FileManager`, `Process`, `UNUserNotificationCenter`. No need to shell out to `osascript` for notifications.
- **No IPC layer needed** -- eliminates the ~400-line socket server (Go) and ~214-line socket client (Swift), the protocol, reconnection logic, handshake versioning, and state synchronization.
- **Single process** -- no daemon + tray coordination, no stale socket cleanup, no "is the daemon running?" state. Simpler launchd setup.
- **SwiftUI potential** -- richer preferences UI, file activity viewer, or rule editor that would be awkward over JSON-over-socket.
- **Better macOS citizen** -- proper app bundle, code signing, notarization, Gatekeeper-friendly distribution.

## Cons of a full Swift rewrite

- **SSH/SFTP is the hardest part** -- Go has `golang.org/x/crypto/ssh` + `pkg/sftp` (mature, battle-tested). Swift has no equivalent. Options: NMSSH, SwiftNIO SSH (no SFTP), or shelling out to `sftp`/`scp`. The current pool with keepalive, reconnection, and SSH agent support (~158 lines of Go) would be significantly harder to replicate.
- **YAML parsing** -- Go has Viper; Swift has no built-in YAML. Would need Yams or switch config format to JSON/plist (breaking existing configs).
- **Loss of Linux support** -- Go cross-compiles to Linux (`task build-linux`). Swift app would be macOS-only.
- **Rewrite cost** -- Go backend is ~5,000 lines across 8 packages with 12 test files. Re-implementing: config validation (23 rules), rule engine, 5 action types, file stabilization, template expansion with 75+ extension mappings.
- **Concurrency model shift** -- Go uses goroutines (one per pending file, channel-based pipeline). Swift async/await can do this but patterns are different and less proven for daemon workloads.
- **No user-facing problem being solved** -- current architecture works. IPC overhead is minimal. Users don't interact with the Go binary directly.

## Config UI: the main blocker

Replacing the YAML config with a SwiftUI settings UI is a medium-to-large effort -- likely more work than the entire Go backend. It requires building a **rule-builder interface** (similar to macOS Mail Rules, Hazel, or Keyboard Maestro).

### What the config expresses

1. **Watch directories** -- list of paths (simple, folder pickers)
2. **SSH hosts** -- name, host, user, optional key path (simple form)
3. **Ignore rules** -- name + match criteria (medium)
4. **Rules** -- the hard part:
   - Ordered list (first match wins, drag-to-reorder)
   - Each rule: name, match criteria (regex, glob, file_type, min/max size, min_age -- all optional, AND-combined)
   - List of actions (polymorphic: move/scp/exec/validate_zip/notify, each with different fields)
   - on_success and on_fail hook lists (also polymorphic actions)

### UI challenges

- **Polymorphic action editing** -- type picker then different form per action type (move needs dest with template tokens, SCP needs host + dest + delete toggle, exec needs command path, etc.)
- **Rule ordering** -- drag-to-reorder with visual indication that first match wins
- **Template tokens** -- dest paths support `{{.Year}}`, `{{.Month}}`, `{{.Type}}`. Needs raw text field or path-builder with token insertion buttons.
- **Regex/glob input** -- needs validation feedback
- **Compound match criteria** -- multiple optional AND-combined fields with clear indication of what's active
- **Nested hooks** -- on_success/on_fail are themselves lists of polymorphic actions

### Storage format

Natural macOS choices: property list via `Codable` + `PropertyListEncoder`, or JSON in `~/Library/Application Support/Avella/`. UserDefaults is less appropriate for this complexity. Core Data/SwiftData would be overkill.

### Scope breakdown

| Component | Effort |
|-----------|--------|
| Data model (`Codable` structs) | Small |
| Persistence (plist/JSON read/write) | Small |
| Settings window with sections | Medium |
| Watch dirs + SSH hosts forms | Small |
| Rule list with reorder | Medium |
| Rule editor with match criteria | Medium |
| Polymorphic action editor | Large |
| Validation and error display | Medium |
| Migration from existing YAML | Small |
