# � AGENTS.md - Go Game Server Utility Library Development Instructions

## � Project Overview
* **Role**: You are an expert Go backend engineer specializing in high-performance utility libraries (like Go-beds, Stretch, or standard library extensions).
* **Goal**: Maintain and extend this Go utility kit. Every tool function must be predictable, side-effect-free, and high-performance.
* **Core Tech Stack**: **Go 1.25+**.


## �️ Setup & Development Commands
* **Format & Lint**: `gofmt -s -w . && golangci-lint run`
* **Race Detection**: `go test -v -race -cover ./...` (Mandatory for concurrent utils)
* **Performance Check**: `go test -bench=. -benchmem ./...` (Run benchmarks for every new utility or optimization)

## � Code Style & Conventions for Utilities

### 1. Stateless & Pure Functions
* **Pure Functions**: Ensure utility functions are deterministic (same input = same output). Do not rely on or modify global state.
* **Concurrency Safe**: All utility functions must be thread-safe and safe for concurrent use by multiple goroutines without locking if possible.

### 2. Modern Go Features (Generics)
* **Type Constraints**: Leverage Go Generics (`any`, `comparable`, or custom constraints from `cmp.Ordered`) to make tools reusable across types.
* **Slice/Map Helpers**: When mutating slices or maps, always consider capacity pre-allocation (`make([]T, 0, cap)`) to avoid redundant memory growing.

### 3. Error Handling & Edge Cases
* **Defensive Programming**: Explicitly handle edge cases for input arguments (e.g., `nil` pointers, empty slices, out-of-bound indices, overflow risks).
* **Predictable Return**: Return a fallback default value alongside an `error` if the operation fails, or use `ok bool` idiom for simple lookups (e.g., `GetOr[T](...) (T, bool)`).

### 4. Performance & Memory (Crucial for Utils)
* **Zero Allocation**: Avoid turning pointer types into interface types unnecessarily, which causes heap escaping.
* **GC Friendly**: For high-frequency utilities (e.g., string manipulation, byte processing), utilize `sync.Pool` or reuse byte slices via `strings.Builder` or `bytes.Buffer`.

## � Boundaries & Don'ts (Red Lines)
* **ABSOLUTELY NO THIRD-PARTY DEPENDENCIES**: The project must remain 100% dependency-free. Only standard library imports are allowed.
* **NO Side Effects**: Do not execute environment operations (like modifying env vars, file writing, or network I/O) unless the utility is explicitly named for that purpose (e.g., `fileutil`).
* **100% Benchmark Coverage**: Every new utility function added **must** have an accompanying benchmark test (`Example_xxx` or `BenchmarkXxx`).
* **Signature Stability**: Do not modify any existing public function names or argument structures to prevent breaking upstream users.
