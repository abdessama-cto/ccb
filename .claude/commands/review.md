# /review — Code review checklist

Review the current changes against:
1. **Correctness**: Does the code do what it's supposed to?
2. **Tests**: Are there tests? Do they cover edge cases?
3. **Security**: Any hardcoded secrets? SQL injection risks? Unvalidated inputs?
4. **Performance**: Any N+1 queries? Unnecessary loops?
5. **Maintainability**: Is the code readable? Are functions too long?
6. **Conventions**: Does it follow the project's coding conventions?

Provide a structured report with: ✅ Good | ⚠️ Concern | ❌ Must fix
