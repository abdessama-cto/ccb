## Git Workflow for `ccb` Development

1.  **Branching Model**: Use a feature-branch workflow. All new features, bug fixes, or improvements should be developed on dedicated branches off `main` (e.g., `feature/add-new-command`, `bugfix/fix-tui-display`).
2.  **Pull Requests (PRs)**: All changes must be submitted via Pull Requests to the `main` branch. PRs should be small, focused, and address a single logical change.
3.  **Code Reviews**: Every PR requires at least one approval from another team member before it can be merged. Reviewers should focus on correctness, adherence to project conventions, code quality, and potential side effects.
4.  **Commit Messages**: Write clear, concise, and descriptive commit messages. Follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification (e.g., `feat: add new 'init' command`, `fix: resolve LLM parsing error`, `docs: update architecture.md`).
5.  **Squash and Rebase**: Before merging, rebase your feature branch onto the latest `main` and squash related commits into logical units. This keeps the `main` branch history clean and linear.
6.  **Avoid Direct Commits to `main`**: Never commit directly to the `main` branch. All changes must go through the PR process.
7.  **Dependency Management**: When adding or updating Go modules, ensure `go mod tidy` is run and `go.mod` and `go.sum` are committed.