# AGENTS.md

## 1. Project Overview

This repository provides reusable Go libraries and infrastructure
components for game server development.

The codebase is intended to support long-running, concurrent,
performance-sensitive server applications.

Prioritize:

1. Correctness and reliability
2. API and behavioral compatibility
3. Simplicity and maintainability
4. Concurrency safety
5. Performance and resource efficiency

Avoid introducing unnecessary complexity, dependencies, or
abstractions.

## 2. Engineering Principles

- Follow idiomatic Go practices.
- Prefer simple and explicit implementations.
- Favor composition over unnecessary abstraction.
- Keep package responsibilities clear and focused.
- Avoid premature optimization.
- Avoid speculative features and unnecessary extensibility.
- Prefer standard library solutions when appropriate.
- Respect existing architectural decisions and conventions.
- Do not refactor unrelated code while implementing a task.

When multiple implementations are possible, prefer the simplest
solution that satisfies the requirements without compromising
correctness, compatibility, or maintainability.

## 3. Repository Exploration

Before modifying code:

1. Read the relevant package and its documentation.
2. Inspect existing interfaces, implementations, and tests.
3. Identify callers and dependencies of the affected code.
4. Understand concurrency and resource ownership requirements.
5. Check for applicable nested AGENTS.md instructions.

Prefer extending existing patterns over introducing new ones.

Do not assume that existing behavior is incorrect without
understanding its purpose.

## 4. Go Coding Standards

- Follow Effective Go and Go Code Review Comments.
- Run gofmt on modified Go files.
- Use goimports when available.
- Use clear, concise, and idiomatic names.
- Keep functions focused and reasonably small.
- Keep interfaces small and consumer-oriented.
- Define interfaces when they provide concrete value.
- Avoid unnecessary interface{} or any usage.
- Prefer concrete types unless abstraction is justified.
- Handle errors explicitly.
- Preserve error chains using appropriate wrapping.
- Avoid panic for recoverable errors.
- Do not silently ignore errors.
- Avoid package-level mutable state unless justified.
- Avoid init() side effects unless required by existing design.
- Document exported APIs and non-obvious invariants.

Do not introduce design patterns solely for architectural
consistency with other programming languages.

## 5. Public API and Compatibility

Treat exported APIs as compatibility-sensitive.

- Preserve existing public APIs unless changes are requested.
- Avoid unnecessary changes to exported types and functions.
- Preserve documented behavior and important edge cases.
- Consider downstream packages and external consumers.
- Avoid introducing breaking changes through refactoring.
- Do not change wire formats or serialized representations
  without explicit requirements.
- Avoid adding exported symbols unnecessarily.
- Prefer internal implementation changes when possible.

Before making an incompatible change, explain:

1. Why the change is necessary.
2. Which consumers may be affected.
3. Whether a compatible alternative exists.
4. What migration steps would be required.

## 6. Concurrency and Goroutine Safety

Concurrency correctness is critical.

- Make shared mutable state synchronization explicit.
- Prefer clear ownership over complex synchronization.
- Avoid data races and unsafe concurrent access.
- Never assume a container is concurrency-safe.
- Define goroutine ownership and termination conditions.
- Ensure background goroutines have a clear shutdown path.
- Avoid goroutine leaks.
- Avoid blocking critical execution paths unnecessarily.
- Avoid holding locks during slow I/O or callbacks.
- Be cautious about lock ordering and deadlocks.
- Do not introduce unbounded goroutine creation.
- Use context.Context for cancellable or deadline-aware
  operations where appropriate.
- Do not store Context in long-lived objects.

For concurrent components, document:

- Ownership of mutable state.
- Synchronization strategy.
- Cancellation and shutdown behavior.
- Whether public methods are safe for concurrent use.

Do not introduce atomic operations, lock-free algorithms,
or additional goroutines without a clear justification.

## 7. Performance and Memory

Game server infrastructure may operate on latency-sensitive
and high-frequency execution paths.

- Avoid unnecessary allocations in hot paths.
- Avoid reflection in performance-critical code unless justified.
- Avoid unnecessary string and byte-slice conversions.
- Avoid excessive temporary objects.
- Consider memory retention and garbage collection pressure.
- Avoid unnecessary copying of large data structures.
- Prefer bounded queues and bounded resource usage.
- Keep synchronization overhead proportional to actual needs.
- Preserve existing performance characteristics when possible.

Do not trade correctness or maintainability for speculative
performance improvements.

Use benchmarks and profiling to justify non-trivial
performance optimizations.

When optimizing code:

1. Identify the relevant workload.
2. Establish a baseline when feasible.
3. Measure allocations and execution time.
4. Compare before and after results.
5. Explain important trade-offs.

## 8. Resource Lifecycle

Long-running game servers require predictable resource cleanup.

- Define ownership of connections, timers, goroutines, and buffers.
- Release resources when they are no longer needed.
- Ensure shutdown paths are safe and predictable.
- Consider repeated initialization and shutdown.
- Avoid resource leaks during partial initialization failures.
- Handle cancellation and timeout scenarios.
- Ensure Close or Stop operations have documented semantics.
- Make repeated Close or Stop calls safe when appropriate.
- Avoid use-after-close behavior.

Do not assume garbage collection is sufficient for managing
external resources or background execution.

## 9. Networking and Protocols

When modifying networking or serialization components:

- Preserve protocol compatibility unless explicitly instructed.
- Validate message lengths and input boundaries.
- Handle malformed and incomplete input safely.
- Avoid unbounded allocations based on untrusted input.
- Handle partial reads and writes correctly.
- Consider connection interruption and timeout behavior.
- Avoid retaining references to reusable buffers unintentionally.
- Make buffer ownership and lifetime explicit.
- Avoid unsafe zero-copy techniques without documented
  lifetime guarantees.

Treat all external network input as untrusted.

Do not silently modify packet framing, byte order,
serialization rules, or message semantics.

## 10. Error Handling and Observability

- Return actionable errors.
- Preserve underlying errors when useful.
- Add context without redundant error wrapping.
- Avoid logging and returning the same error at every layer.
- Do not suppress unexpected failures.
- Avoid excessive logging in hot paths.
- Never log secrets or sensitive credentials.
- Keep operational diagnostics useful and concise.

Libraries should not terminate the process for ordinary
runtime errors.

Prefer returning errors to callers rather than enforcing
application-level logging or recovery policies.

## 11. Dependencies

- Prefer the Go standard library when it meets requirements.
- Reuse existing dependencies where appropriate.
- Avoid introducing dependencies for trivial functionality.
- Consider dependency maintenance and compatibility.
- Avoid unnecessary dependency upgrades.
- Do not modify go.mod or go.sum without a clear reason.
- Keep package dependency directions consistent with the
  existing architecture.
- Avoid import cycles.

New dependencies should provide clear value relative to
their maintenance and compatibility costs.

## 12. Testing Requirements

All behavior changes should be validated.

- Add or update tests for modified behavior.
- Prefer deterministic and isolated tests.
- Include edge cases and failure scenarios.
- Test concurrency behavior where applicable.
- Test cancellation and shutdown behavior.
- Avoid arbitrary sleeps in concurrent tests.
- Avoid dependence on external services unless required.
- Add regression tests for bug fixes.
- Add benchmarks for performance-sensitive changes.
- Consider fuzz tests for parsers, codecs, and untrusted input.

Common verification commands:

    go test ./...
    go test -race ./...
    go vet ./...

Use targeted package tests during development.

Run broader validation when the scope of the change
justifies it and the environment supports it.

If a verification step cannot be completed, report
the limitation explicitly.

## 13. Change Scope and Review

Keep changes minimal and focused.

- Modify only files relevant to the task.
- Avoid unrelated cleanup and formatting changes.
- Do not rename or reorganize packages without justification.
- Preserve existing tests unless behavior intentionally changes.
- Do not weaken tests merely to make them pass.
- Avoid broad architectural refactoring for localized changes.
- Do not modify generated files directly when a generator exists.
- Do not overwrite unrelated user changes.

Before completing a task, review the diff for:

1. Unintended API changes.
2. Concurrency risks.
3. Resource leaks.
4. Error-handling regressions.
5. Performance regressions.
6. Missing tests.
7. Unnecessary dependencies.

## 14. Agent Working Guidelines

For non-trivial changes:

1. Understand the current implementation.
2. Identify relevant constraints and compatibility risks.
3. Propose a concise implementation approach.
4. Implement the smallest correct change.
5. Add or update relevant tests.
6. Run applicable verification.
7. Review the final diff.

Do not invent missing requirements.

When requirements are ambiguous, prefer preserving existing
behavior and making assumptions explicit.

For architectural or compatibility-sensitive decisions,
explain the trade-offs before making significant changes.

When completing a task, summarize:

- What changed.
- Why it changed.
- What was verified.
- Any remaining risks or limitations.