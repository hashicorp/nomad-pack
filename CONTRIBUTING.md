# CONTRIBUTING to Nomad Pack

We welcome contributions to Nomad Pack.

To add packs, contribute to the [Nomad Pack Community Registry](https://github.com/hashicorp/nomad-pack-community-registry).

## Development dependencies

- Go (see [`.go-version`](.go-version) for the required version)
- Git
- Make

Ensure `$GOPATH/bin` is on your `$PATH`.

## Build and run locally

Install required tools:

```shell-session
$ make bootstrap
```

Check `go.mod` and `go.sum`:

```shell-session
$ make check
```

Build a binary from local code. The binary is output to `./bin/nomad-pack`:

```shell-session
$ make dev
```

Run the binary:

```shell-session
$ ./bin/nomad-pack -h
```

## Common make targets

| Target | Purpose |
|---|---|
| `make bootstrap` | Install all development tools |
| `make check` | Verify Go mod is tidy |
| `make dev` | Build binary to `./bin/nomad-pack` |
| `make test` | Run the full test suite |
| `make lint` | Lint source code with golangci-lint |

Run `make help` to see all available targets.


