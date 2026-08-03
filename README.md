# krypticdev

```bash
go get github.com/dev-kryptic/krypticdev
```

```go
import "github.com/dev-kryptic/krypticdev"

func main() {
    krypticdev.Inject() // populates the process environment in development
    // ... rest of application
}
```

No-op outside development (`GO_ENV`/`APP_ENV`/`ENVIRONMENT` = production/staging, or
`KRYPTIC_DISABLED=true`). Finds `kryptic.json` by walking up from the working directory,
never overwrites existing environment variables, never panics. Configuration:
`KRYPTIC_PROJECT_ID`, `KRYPTIC_ENV`, `KRYPTIC_SOCKET_PATH`, `KRYPTIC_TIMEOUT_MS`, `KRYPTIC_SILENT`.

Protocol: [daemon/PROTOCOL.md](https://github.com/dev-kryptic/Kryptic.Daemon/blob/main/PROTOCOL.md). License: Apache-2.0. `go test ./...`
