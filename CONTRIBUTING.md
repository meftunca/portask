# Contributing to Portask

First off, thank you for considering contributing to Portask! 🎉

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct. Please be respectful and constructive in all interactions.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the existing issues to avoid duplicates. When creating a bug report, include:

- **Clear description** of the issue
- **Steps to reproduce** the behavior
- **Expected vs actual behavior**
- **Environment details** (OS, Go version, etc.)
- **Logs or error messages**
- **Minimal reproducible example** if possible

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion:

- Use a **clear and descriptive title**
- Provide a **detailed description** of the proposed feature
- Explain **why this enhancement would be useful**
- Include **code examples** if applicable

### Pull Requests

1. **Fork the repository** and create your branch from `main`

   ```bash
   git checkout -b feature/amazing-feature
   ```

2. **Make your changes** following our coding standards

3. **Add tests** for your changes

   - Aim for >70% coverage
   - Include unit tests and integration tests where appropriate

4. **Run the test suite**

   ```bash
   make test
   make test-coverage
   ```

5. **Run linters**

   ```bash
   make lint
   make format
   ```

6. **Commit your changes** with clear commit messages

   ```bash
   git commit -m "feat: add amazing feature"
   ```

   We follow [Conventional Commits](https://www.conventionalcommits.org/):

   - `feat:` New feature
   - `fix:` Bug fix
   - `docs:` Documentation changes
   - `test:` Adding or updating tests
   - `refactor:` Code refactoring
   - `perf:` Performance improvements
   - `chore:` Maintenance tasks

7. **Push to your fork**

   ```bash
   git push origin feature/amazing-feature
   ```

8. **Open a Pull Request** with:
   - Clear description of changes
   - Link to related issues
   - Screenshots/GIFs for UI changes
   - Performance benchmarks if applicable

## Development Setup

### Prerequisites

- Go 1.23+
- Docker & Docker Compose
- Make
- Git

### Getting Started

1. Clone the repository:

   ```bash
   git clone https://github.com/meftunca/portask.git
   cd portask
   ```

2. Install dependencies:

   ```bash
   make deps
   ```

3. Start development environment:

   ```bash
   docker-compose up -d
   ```

4. Run tests:

   ```bash
   make test
   ```

5. Build the project:
   ```bash
   make build
   ```

### Project Structure

```
portask/
├── cmd/              # Application entrypoints
├── pkg/              # Library code
│   ├── api/         # HTTP/WebSocket API
│   ├── queue/       # Queue implementation
│   ├── storage/     # Storage backends
│   ├── network/     # Network protocols
│   └── ...
├── configs/          # Configuration files
├── docs/            # Documentation
├── examples/        # Example code
├── tests/           # Integration tests
└── benchmarks/      # Performance benchmarks
```

## Coding Standards

### Go Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Run `gofmt` and `goimports` before committing
- Use meaningful variable and function names
- Add comments for exported functions and types
- Keep functions small and focused
- Avoid premature optimization

### Testing Guidelines

- Write table-driven tests where appropriate
- Use descriptive test names: `TestFunctionName_Scenario_ExpectedBehavior`
- Test both success and failure cases
- Mock external dependencies
- Benchmark performance-critical code

Example test:

```go
func TestQueue_Enqueue_Success(t *testing.T) {
    tests := []struct {
        name     string
        message  *types.PortaskMessage
        wantErr  bool
    }{
        {
            name: "enqueue valid message",
            message: &types.PortaskMessage{
                ID:      "test-1",
                Payload: []byte("test"),
            },
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            q := NewQueue(100)
            err := q.Enqueue(tt.message)
            if (err != nil) != tt.wantErr {
                t.Errorf("Enqueue() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Performance Considerations

When contributing performance-sensitive code:

1. **Benchmark first**: Add benchmarks for your changes

   ```bash
   make benchmark
   ```

2. **Profile if needed**: Use pprof for CPU/memory profiling

   ```bash
   make profile
   ```

3. **Avoid allocations**: Use object pools for frequently allocated objects

4. **Lock-free when possible**: Prefer atomic operations over mutexes

5. **Document trade-offs**: Explain performance vs readability choices

## Review Process

1. **Automated checks** must pass:

   - Tests
   - Linters
   - Security scans
   - Coverage threshold

2. **Code review** by maintainers:

   - Code quality
   - Test coverage
   - Documentation
   - Performance impact

3. **Performance verification** for critical paths:
   - Benchmark results
   - Memory profiling
   - Load testing if applicable

## Release Process

Maintainers will:

1. Update CHANGELOG.md
2. Create a release tag
3. Build and publish artifacts
4. Update documentation

## Questions?

Feel free to:

- Open an issue for discussion
- Join our community channels
- Email the maintainers

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to Portask! 🚀
