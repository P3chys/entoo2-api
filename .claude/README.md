# Claude Code Configuration

This directory contains Claude Code v2.1.0+ configuration for the Entoo2 API project.

## Quick Start

```bash
cd D:\Entoo2\entoo2-api
claude
/build
/run
```

## Available Skills

- `/build` - Build the API server
- `/run` - Run in development mode
- `/test` - Run all tests with coverage
- `/lint` - Run linter checks

## Documentation

- **[CLAUDE.md](./CLAUDE.md)** - Project context and conventions
- **[Parent Usage Guide](../entoo2-infra/.claude/USAGE_GUIDE.md)** - Full Claude Code documentation

## Files

- `settings.json` - Permissions configuration
- `skills/` - Custom skills (auto-reload on change)
- `CLAUDE.md` - Project context for Claude

## v2.1.0 Features

This configuration uses:
- ✅ Skill hot-reload
- ✅ Wildcard permissions
- ✅ Skill frontmatter metadata

See [parent project's usage guide](../entoo2-infra/.claude/USAGE_GUIDE.md) for comprehensive documentation.
