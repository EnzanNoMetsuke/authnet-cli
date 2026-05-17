# Use Go for the CLI

The Authorize.Net operations CLI will be implemented in Go. Go was chosen over Rust and TypeScript because it provides native cross-platform binaries, straightforward release automation, strong CLI ergonomics, and a Go-native path to richer terminal interfaces, even though Authorize.Net does not publish an official Go SDK.

## Considered Options

- Go: chosen for native distribution, implementation speed, release tooling, and terminal UI ecosystem fit.
- Rust: rejected because the extra implementation complexity is not justified for this CLI's expected performance needs.
- TypeScript: rejected because a Node runtime or bundled runtime would weaken the native binary distribution story.

## Consequences

The project will call the Authorize.Net API directly rather than wrapping an official Go SDK.
