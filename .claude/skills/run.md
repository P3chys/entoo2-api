---
name: run
description: Run the API server in development mode
category: development
context: default
---

Run the Entoo2 API server in development mode.

First check if there's a Makefile target:

```bash
make run
```

If that doesn't work, run directly:

```bash
go run cmd/server/main.go
```

The server should start on the configured port (usually 8080).
Report when the server is ready and listening.
