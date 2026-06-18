# Contributing to Go Blockchain

First off, thank you for considering contributing to the `go-blockchain` project! It's people like you that make it such a great educational tool and showcase project.

## 🧑‍💻 How to Contribute

### 1. Reporting Bugs
This section guides you through submitting a bug report. Following these guidelines helps maintainers and the community understand your report, reproduce the behavior, and find related reports.
- Use the GitHub Issues tab to report a bug.
- Provide a clear and descriptive title for the issue.
- Describe the exact steps to reproduce the problem.
- Provide the Go version (`go version`) you are using and the OS.

### 2. Suggesting Enhancements
Enhancement suggestions are tracked as GitHub issues.
- Provide a clear and descriptive title for the issue.
- Explain why this enhancement would be useful to most users.
- Outline the proposed architecture or code changes if you have any in mind.

### 3. Pull Requests
We actively welcome your pull requests! 
1. **Fork the repository** and create your branch from `main`.
2. **Install dependencies:** `go mod tidy` and `go mod download`.
3. **If you've added code**, ensure you add comprehensive tests! We aim to maintain a high level of test coverage.
4. **Ensure the test suite passes:** `go test ./...`
5. **Lint your code:** Run `golangci-lint run` locally before submitting.
6. **Format your code:** Run `go fmt ./...`.
7. **Create the PR** detailing your changes. 

## 🏗 Development Guidelines

### Architecture Principles
- **Decoupling**: Maintain strict boundaries between the `core`, `network`, `storage`, and `cli` layers. Don't introduce cyclical dependencies.
- **Dependency Injection**: Write testable code. Provide interfaces and abstract implementations whenever IO (Network, DB, Stdout) is involved (e.g., passing `io.Writer` in CLI commands).
- **Go 1.25 Features**: Embrace modern Go idioms, such as `range-over-func` iterators where applicable.

### Testing Strategy
- **Unit Tests**: Test core domain logic in isolation.
- **Integration Tests**: Test the intersections between `network`, `core`, and `storage` (using BoltDB temp files or in-memory networks).
- **End-to-End Tests**: Add e2e commands testing to `internal/cli/root_test.go` to ensure flags and user experiences remain stable.

We look forward to your contributions!
