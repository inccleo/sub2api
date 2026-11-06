# Building a custom release

The release includes a complete source archive from its tagged Git commit,
including the maintained BEpusdt engine and SQLite adapter under
`backend/third_party`. Preserve `LICENSE`, `LICENSES`, and
`THIRD_PARTY_NOTICES.md` when redistributing the combined server.

Use Node.js 22 or later, pnpm 9, and the Go toolchain specified by
`backend/go.mod`. From the extracted source root:

```sh
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend run build
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -C backend \
  -tags=embed -trimpath -o ../dist/sub2api ./cmd/server
```

The frontend build writes `backend/internal/web/dist`, which the server embeds.
The release manifest records the exact version and source commit. The custom
workflow adds version, commit, UTC build date, and release type using Go linker
flags; see `.github/workflows/custom-release.yml` for those flags and packaging.

Embedded collection engine tests use the parent module's dependencies:

```sh
cd backend
go test github.com/v03413/bepusdt/app/conf github.com/v03413/bepusdt/app/task
go test -tags unit ./internal/bepusdt ./internal/payment/provider
```
