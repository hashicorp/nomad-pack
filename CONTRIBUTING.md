# CONTRIBUTING to Nomad Pack

## Architecture

Nomad Pack is structured around four layers: CLI commands, a pack manager, a
renderer/variable pipeline, and a runner that communicates with the Nomad API.

### Package overview

| Package | Responsibility |
|---|---|
| `internal/cli` | Command parsing, flag handling, user-facing orchestration |
| `internal/pkg/manager` | Load pack trees, coordinate variable parsing and rendering |
| `internal/pkg/renderer` | Execute Go templates against resolved variables |
| `internal/pkg/variable/parser` | Parse variable definitions, apply overrides, validate |
| `internal/pkg/variable/source` | External variable sources: Consul, Vault, and Nomad Variables |
| `internal/pkg/caching` | Local registry and pack cache backed by Git |
| `internal/runner` | `Runner` interface — deploy, plan, destroy |
| `internal/runner/job` | Concrete runner for Nomad job objects |
| `sdk/pack` | `Pack` and `Metadata` structs — the public pack model |
| `sdk/pack/variables` | `Variable` type with HCL2 type constraints and validations |
| `terminal` | `UI` interface — console, non-interactive, and live-view outputs |

### Key types

```mermaid
classDiagram
    class Pack {
        -Path string
        -Metadata Metadata
        -TemplateFiles File[]
        -dependencies Pack[]
        -parent Pack
        -alias string
        +Name() string
        +Dependencies() Pack[]
        +AddDependency(ID, Pack)
    }

    class Metadata {
        +App MetadataApp
        +Pack MetadataPack
        +Dependencies Dependency[]
    }

    class Dependency {
        +Name string
        +Alias string
        +Source string
        +Ref string
    }

    class Variable {
        +Name ID
        +Default cty.Value
        +Type cty.Type
        +Value cty.Value
        +Validations Validation[]
        +Validate() Diagnostics
    }

    class PackManager {
        -cfg Config
        -client api.Client
        -renderer Renderer
        -loadedPack Pack
        +ProcessVariableFiles()
        +ProcessTemplates() Rendered
    }

    class Renderer {
        +PackPath string
        +Client api.Client
        +Render(Pack, ParsedVariables) Rendered
    }

    class Rendered {
        -parentRenders map
        -dependencyRenders map
        -parsedVariables ParsedVariables
        +ParentRenders() map
        +DependentRenders() map
    }

    class ParsedVariables {
        -v2Vars map
        -nomadVars map
        +GetVars() map
        +ToPackTemplateContext() map
    }

    class Runner {
        <<interface>>
        +SetTemplates(map)
        +ParseTemplates() errors[]
        +CanonicalizeTemplates() errors[]
        +Deploy() error
        +PlanDeployment() int
        +DestroyDeployment() errors[]
    }

    class JobRunner {
        -cfg CLIConfig
        -client api.Client
        -parsedTemplates map
        -evalIDs string[]
        +Deploy() error
        +PlanDeployment() int
    }

    class baseCommand {
        +Ctx context.Context
        +ui terminal.UI
        +vars map
        +varFiles string[]
        +Init(opts...)
        +getAPIClient() api.Client
    }

    Pack --> Metadata
    Metadata --> Dependency
    Pack --> Pack : parent / dependencies
    PackManager --> Pack
    PackManager --> Renderer
    Renderer --> Rendered
    Rendered --> ParsedVariables
    ParsedVariables --> Variable
    Runner <|.. JobRunner
    baseCommand --> PackManager
    baseCommand --> Runner
```

### Command interaction: `nomad-pack run`

The following sequence traces one complete `run` invocation from user input through
to a deployed Nomad job.

```mermaid
sequenceDiagram
    actor User
    participant CLI as RunCommand
    participant Manager as PackManager
    participant Parser as VarParser
    participant Renderer
    participant Runner as JobRunner
    participant Nomad as Nomad API

    User->>CLI: nomad-pack run my-pack --var env=prod
    CLI->>CLI: Init - parse flags, create UI
    CLI->>CLI: getAPIClient()

    CLI->>Manager: NewPackManager(config)
    CLI->>Manager: ProcessTemplates()

    Manager->>Manager: loadAndValidatePacks()
    Note over Manager: Resolve pack tree recursively

    Manager->>Parser: NewParser(config)
    Parser->>Parser: Parse variable definitions
    Parser->>Parser: Apply overrides - env then file then CLI args
    Parser->>Parser: Validate types and constraints
    Parser-->>Manager: ParsedVariables

    Manager->>Renderer: Render(pack, ParsedVariables)
    Renderer->>Renderer: prepareFiles - collect templates
    Renderer->>Renderer: Execute Go templates
    Renderer-->>Manager: Rendered

    Manager-->>CLI: Rendered

    CLI->>Runner: NewDeployer(client, config)
    CLI->>Runner: SetTemplates(rendered)
    CLI->>Runner: ParseTemplates()
    Note over Runner: Validate HCL via Nomad API
    CLI->>Runner: CanonicalizeTemplates()
    CLI->>Runner: CheckForConflicts()

    CLI->>Runner: Deploy(ui, errorContext)
    loop for each job template
        Runner->>Nomad: Jobs.RegisterOpts(job)
        Nomad-->>Runner: EvalID
    end
    Runner->>Runner: stopRemovedJobs
    Runner-->>CLI: success

    CLI->>Nomad: Variables.Create or Update
    CLI->>Nomad: Evaluations.Info - poll until done
    CLI-->>User: Pack deployed
```

### Variable resolution order

Variables are resolved in the following order, with later sources winning on
conflict:

1. Default values declared in `variables.hcl`
2. External sources — Consul KV, Vault secrets, and Nomad Variables (in source order)
3. Variable files passed with `-f` / `--var-file` (left to right)
4. Individual `--var key=value` flags

## Code contributions

We welcome contributions to Nomad Pack.

To add packs, contribute to the [Nomad Pack Community Registry](https://github.com/hashicorp/nomad-pack-community-registry).

### Development dependencies

- Go (see [`.go-version`](.go-version) for the required version)
- Git
- Make

Ensure `$GOPATH/bin` is on your `$PATH`.

### Build and run locally

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

### Common make targets

| Target | Purpose |
|---|---|
| `make bootstrap` | Install all development tools |
| `make check` | Verify Go mod is tidy |
| `make dev` | Build binary to `./bin/nomad-pack` |
| `make test` | Run the full test suite |
| `make lint` | Lint source code with golangci-lint |

Run `make help` to see all available targets.


