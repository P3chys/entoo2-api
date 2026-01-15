---
name: build
description: Build the Go API server
category: development
context: default
---

Build the Entoo2 API server.

Run the following command:

```bash
make build
```

If no Makefile target exists, use:

```bash
go build -o bin/server cmd/server/main.go
```

Report the build status and any errors.
