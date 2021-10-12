# Contributing to Nomad Pack

Contributions to Nomad Pack are welcome.

To add packs, please contribute to the [Nomad Pack Community Registry](https://github.com/hashicorp/nomad-pack-community-registry).

## Development dependencies

- Golang
- Git
- Make

## Building and Running Locally

Check Go mod and Go sum:

```
make check
```

Build a binary from local code. This will add an
executable at `./bin/nomad-pack`:

```
make dev
```

Run your code:

```
./bin/nomad-pack -h
```
