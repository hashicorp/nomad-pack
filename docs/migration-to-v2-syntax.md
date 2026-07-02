# Migrating Packs to the Current Template Syntax

This guide will help you update a pack from the template syntax used by earlier versions of Nomad Pack to the current syntax. Current templates read a pack's variables and metadata through template functions, while earlier versions accessed those values as fields on the template context.

For an overview of writing packs, see the [Writing Packs Guide](./writing-packs.md). For the full list of template functions, see the [Functions Guide](./functions.md).

## Accessing Variables

Earlier versions accessed pack variables as fields on the template context, either through the `.my` alias or the pack's name. To read a variable now, call the `var` function with the variable name and the context to read it from.

For example, a template that reads a variable named `message` changes from `[[ .my.message ]]` to:

```
[[ var "message" . ]]
```

The trailing `.` passes the current context to the function so it knows which pack to read from.

The original syntax reached nested values with additional dots. Those become a single dotted key, so `[[ .my.resources.cpu ]]` becomes:

```
[[ var "resources.cpu" . ]]
```

A dotted key only works when the parent variable is an object, so this example assumes `resources` is an object variable with a `cpu` field.

To read every variable for a pack at once, use the `vars` function:

```
[[ vars . ]]
```

When a variable holds a collection, call `var` wherever the original template used the field, including as the target of a `range`. A loop over an `env` map changes from:

```
[[ range $k, $v := .my.env ]][[ $k ]] = [[ $v | quote ]]
[[ end ]]
```

to:

```
[[ range $k, $v := var "env" . ]][[ $k ]] = [[ $v | quote ]]
[[ end ]]
```

## Accessing Metadata

Earlier versions accessed pack metadata through the `.nomad_pack` field. To read metadata now, call the `meta` function, which works the same way as `var`. A reference to the pack name changes from `[[ .nomad_pack.pack.name ]]` to:

```
[[ meta "pack.name" . ]]
```

## Accessing Dependencies

The original syntax put every pack's values on a single shared context. When a pack had dependencies, particularly aliased or nested ones, it was hard to tell which pack a value came from. The current syntax gives each dependency its own context, so it is always clear which pack you are reading from.

Each dependency is a field on the parent context. Pass it to `var` or `meta` in place of the current context.

```
[[ var "job_name" . ]]
[[ var "job_name" .child ]]
```

Here `.` reads the `job_name` variable from the current pack and `.child` reads the `job_name` variable from a dependency named `child`. Dependencies can nest, and the `deps` function returns a pack's direct dependencies so you can loop over them.

```
[[ range $dep := deps . ]][[ var "job_name" $dep ]]
[[ end ]]
```

## A Complete Example

The following jobspec template uses the original syntax.

```
job [[ coalesce .simple_raw_exec.job_name .nomad_pack.pack.name | quote ]] {
  datacenters = [[ .simple_raw_exec.datacenters | toJson ]]
  type        = "service"

  group "app" {
    count = [[ .simple_raw_exec.count ]]

    task "server" {
      driver = "raw_exec"

      config {
        command = "/bin/bash"
        args    = ["-c", [[ .simple_raw_exec.command | quote ]]]
      }

      resources {
        cpu    = [[ .simple_raw_exec.resources.cpu ]]
        memory = [[ .simple_raw_exec.resources.memory ]]
      }
    }
  }
}
```

The same template using the current syntax.

```
job [[ coalesce (var "job_name" .) (meta "pack.name" .) | quote ]] {
  datacenters = [[ var "datacenters" . | toJson ]]
  type        = "service"

  group "app" {
    count = [[ var "count" . ]]

    task "server" {
      driver = "raw_exec"

      config {
        command = "/bin/bash"
        args    = ["-c", [[ var "command" . | quote ]]]
      }

      resources {
        cpu    = [[ var "resources.cpu" . ]]
        memory = [[ var "resources.memory" . ]]
      }
    }
  }
}
```

## Helper and Output Templates

The same rules apply everywhere a pack renders templates, not only in jobspec files. Output templates (`outputs.tpl`) and named helper templates defined with `define` share the same context and functions.

A helper that receives the pack context reads variables and metadata just like the main template. A helper that sets a job's name changes from:

```
[[ define "job_name" ]]
[[- if eq .my.job_name "" -]]
[[- .nomad_pack.pack.name | quote -]]
[[- else -]]
[[- .my.job_name | quote -]]
[[- end ]]
[[- end ]]
```

to:

```
[[ define "job_name" ]]
[[- if eq (var "job_name" .) "" -]]
[[- meta "pack.name" . | quote -]]
[[- else -]]
[[- var "job_name" . | quote -]]
[[- end ]]
[[- end ]]
```

Invoke the helper with the pack context so the functions can resolve values:

```
job [[ template "job_name" . ]] {
```

A helper that receives a plain value rather than the pack context does not change, because it reads the fields of whatever value you pass in. You update only the invocation to source that value with `var`, so `[[ template "resources" .my.resources ]]` becomes:

```
[[ template "resources" (var "resources" .) ]]
```

## Running Legacy Packs

You can still run a pack you have not migrated yet by passing the `--parser-v1` flag, which parses it with the original syntax. The commands that parse pack templates accept the flag: `run`, `plan`, `render`, `info`, `stop`, and `destroy`.

```
nomad-pack run ./my-pack --parser-v1
```

Treat `--parser-v1` as a temporary measure while you update a pack. Write new packs with the current syntax.

## Troubleshooting

When you run a pack that still uses the original syntax without the `--parser-v1` flag, Nomad Pack reports the field it could not find and suggests the equivalent function call.

```
The legacy ".my.message" syntax should be updated to use `var "message" .`. You can run legacy packs unmodified by using the `--parser-v1` flag
```

The opposite happens when you run a pack that uses the current syntax with the `--parser-v1` flag. The template functions are unavailable, so Nomad Pack reports that the function is not implemented for the v1 syntax. Remove the `--parser-v1` flag to parse the pack with the current syntax.

### Missing Values

The `var` and `meta` functions return an empty string when a key is not found, so a misspelled name renders as empty instead of raising an error. While you migrate, use `must_var` and `must_meta` instead, which stop rendering and report the missing key.

```
[[ must_var "message" . ]]
```
