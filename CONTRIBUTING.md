# Contributing to K8s Ingress Meta Sync

Thank you for your interest in contributing to the K8s Ingress Meta Sync project! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please read and follow our [Code of Conduct](CODE_OF_CONDUCT.md) to keep our community approachable and respectable.

## Getting Started

1. **Fork the Repository**: Start by forking the repository to your GitHub account.

2. **Clone Your Fork**: Clone your fork to your local machine.
   ```bash
   git clone https://github.com/your-username/k8s-ingress-meta-sync.git
   cd k8s-ingress-meta-sync
   ```

3. **Add Upstream Remote**: Add the original repository as an upstream remote.
   ```bash
   git remote add upstream https://github.com/original-owner/k8s-ingress-meta-sync.git
   ```

## Development Workflow

1. **Create a Branch**: Create a new branch for your feature or bugfix.
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make Changes**: Implement your changes, following the coding standards and guidelines.

3. **Write Tests**: Add tests for your changes to ensure they work as expected.

4. **Run Tests**: Make sure all tests pass before submitting a pull request.
   ```bash
   go test ./...
   ```

5. **Commit Changes**: Commit your changes with a clear and descriptive commit message.
   ```bash
   git commit -m "Add feature: description of your changes"
   ```

6. **Push Changes**: Push your changes to your fork.
   ```bash
   git push origin feature/your-feature-name
   ```

7. **Submit a Pull Request**: Create a pull request from your branch to the main repository.

## Pull Request Guidelines

1. **Title**: Use a clear and descriptive title for your pull request.

2. **Description**: Provide a detailed description of your changes, including the motivation behind them.

3. **Reference Issues**: Reference any related issues using GitHub's syntax (e.g., "Fixes #123").

4. **Scope**: Keep pull requests focused on a single feature or bug fix.

5. **Tests**: Ensure all tests pass and include new tests for your changes.

6. **Documentation**: Update documentation to reflect your changes.

## Coding Standards

1. **Go Code**:
   - Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
   - Use `gofmt` to format your code
   - Document exported functions, types, and constants

2. **Kubernetes Resources**:
   - Follow Kubernetes [API conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
   - Use CamelCase for resource names
   - Include comprehensive documentation for custom resources

## Adding New Providers

To add a new IP range provider:

1. Create a new directory under `pkg/providers/` for your provider
2. Implement the `Provider` interface defined in `pkg/providers/provider.go`
3. Register your provider in the provider registry in your init function
4. Add tests for your provider
5. Update documentation to include your new provider

## Adding New Ingress Types

To add a new ingress type:

1. Create a new directory under `pkg/ingress/` for your ingress type
2. Implement the `Ingress` interface defined in `pkg/ingress/ingress.go`
3. Register your ingress type in the ingress registry in your init function
4. Add tests for your ingress type
5. Update documentation to include your new ingress type

## Testing

1. **Unit Tests**: Write unit tests for all code changes.
   ```bash
   go test ./...
   ```

2. **Integration Tests**: Run integration tests if your changes affect interaction with external systems.
   ```bash
   make test-integration
   ```

3. **End-to-End Tests**: Run end-to-end tests to validate the complete workflow.
   ```bash
   make test-e2e
   ```

## Documentation

- Update documentation to reflect any changes or additions
- Use clear and concise language
- Include examples where appropriate
- Update diagrams if necessary

## License

By contributing to this project, you agree that your contributions will be licensed under the project's [MIT License](LICENSE).

## Questions

If you have any questions or need help, please open an issue or reach out to the maintainers.

Thank you for contributing to K8s Ingress Meta Sync!
