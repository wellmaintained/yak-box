# yak-box

Docker-based worker orchestration CLI for managing sandboxed and native workers.

## Build

```bash
go build -o yak-box .
```

## What it does

yak-box spawns, manages, and stops containerized worker environments. It provides commands for:

- **spawn** - Start a new worker (sandboxed via Docker or native)
- **stop** - Stop a running worker
- **check** - Verify environment and prerequisites
- **message** - Send messages to workers
