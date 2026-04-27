# Nomad Pack functions

The nomad-pack template renderer contains various helper functions used while rendering
templates.

Nomad Pack provides all the [Sprig template functions][Sprig] for text manipulation.

Nomad Pack also provides additional functions for accessing Nomad, testing IP addresses, and
template debugging.

## Functions by topic

- [Nomad API][topicNomadAPI]
- [Debugging][topicDebugging]
- [Helpers][topicHelpers]

You can also view [the list of functions in alphabetical order][fnByAlpha]

## Nomad API <a id="topicNomadAPI"></a>

### Namespace functions

#### `nomadNamespace` <a id="nomadNamespace"></a>

The `nomadNamespace` returns the details for the namespace with the given name.

**NOTE:** The `nomadNamespace` function will error on missing namespaces, which
will prevent the template from rendering. Any part of the template that uses
the `nomadNamespace` function should be using a namespace name provided by the
`nomadNamespaces` function for additional safety.

##### Parameters

- 1: `string` - The target namespaces's name

##### Returns

- `error` or [`*api.Namespace`][] for the requested namespace.

##### Example

Get the name and description for each template accessible to the current user.

```
[[ range $ns := nomadNamespaces]]
    [[ with nomadNamespace $ns ]]
      [[printf "%s: %s\n" .Name .Description ]]
    [[ end ]]
[[ end ]]
```

#### `nomadNamespaces` <a id="nomadNamespaces"></a>

Retrieve a list of namespaces visible to the current user.

##### Parameters

- None

##### Returns

- `error` or \[][`*api.Namespace`][].

##### Example

```
[[- range nomadNamespaces -]]
    [[- printf "%v: %v\n" .Name .Description -]]
[[- end -]]
```

```
default: Default shared namespace
```

### Variable functions

#### `nomadVariables` <a id="nomadVariables"></a>

The `nomadVariables` function retrieves a list of all Nomad Variables stored in the specified namespace.

##### Parameters

- 1: `string` - The target namespace name
- 2: `string` (optional) - Prefix to filter variables by path

##### Returns

- `error` or `[]*api.VariableMetadata` - A list of Variable metadata objects (path, namespace, timestamps, lock info). Does not include actual key-value items.

##### Example

List all variables in a namespace:

[[ range nomadVariables "production" ]]
Path: [[ .Path ]]
Namespace: [[ .Namespace ]]
Modified: [[ .ModifyTime ]]
[[ end ]]

Filter variables by prefix:

[[ range nomadVariables "production" "secret/" ]]
Path: [[ .Path ]]
[[ end ]]

Get variable data:

[[ $meta := index (nomadVariables "production") 0 ]]
[[ $var := nomadVariable $meta.Path "production" ]]
Password: [[ $var.Items.password ]]


#### `nomadVariable` <a id="nomadVariable"></a>

The `nomadVariable` function retrieves a specific Nomad Variable by path and namespace.

##### Parameters

- 1: `string` - The path of the variable
- 2: `string` - The namespace

##### Returns

- `error` or `*api.Variable` - The Variable object

##### Example

[[ with nomadVariable "secret/db" "production" ]]
password = "[[ .Items.password ]]"
[[ end ]]

### Consul KV functions

#### `consulKey` <a id="consulKey"></a>

Retrieves a single key-value pair from Consul's KV store.

##### Parameters

- 1: `string` - The key path in Consul KV

##### Returns

- `string` - The value stored at the specified key, or empty string if not found

##### Example

[[ $dbPassword := consulKey "config/database/password" ]]
password = "[[ $dbPassword ]]"

#### `consulKeys` <a id="consulKeys"></a>

Retrieves multiple key-value pairs from Consul's KV store with a given prefix.

##### Parameters

- 1: `string` - The key prefix to search for

##### Returns

- `map[string]string` - A map where keys are the full Consul KV paths and values are the stored values

##### Example

List all configuration values with a prefix:

[[ range $key,$value := consulKeys "config/app/" ]]
[[ $key ]] = [[ $value ]]
[[ end ]]

Access specific keys from the result:

[[ $configs := consulKeys "config/app/" ]][[ $appName := index $configs "config/app/name" ]]
app_version = "[[ index $configs "config/app/version" ]]"

Iterate and filter:

[[ $configs := consulKeys "config/" ]][[ range $key, $value :=$configs ]]
[[ if hasPrefix $key "config/prod/" ]][[ $key ]]: [[ $value ]]
[[ end ]]
[[ end ]]

#### Consul KV Configuration

The Consul KV template functions (`consulKey` and `consulKeys`) require configuration to connect to Consul. Configuration is provided via environment variables, with optional TLS settings available via CLI flags.

**Client Creation:** Nomad Pack will attempt to create a Consul API client when:
- The `CONSUL_HTTP_ADDR` environment variable is set

The Consul client uses `consulapi.DefaultConfig()` which automatically loads all standard `CONSUL_*` environment variables.

##### CLI Flags

The following flags are available for `nomad-pack run`, `nomad-pack plan`, and `nomad-pack render` commands:

- `--consul-address` - Consul server address (e.g., `https://consul.example.com:8501`)
- `--consul-token` - Consul ACL token for authentication
- `--consul-namespace` - Consul namespace (Consul Enterprise only)
- `--consul-ca-cert` - Path to CA certificate file for TLS verification
- `--consul-client-cert` - Path to client certificate file for mutual TLS (mTLS)
- `--consul-client-key` - Path to client private key file for mutual TLS (mTLS)
- `--consul-tls-skip-verify` - Skip TLS certificate verification (not recommended for production)
- `--consul-tls-server-name` - Server name to use for TLS SNI (Server Name Indication)

##### Environment Variables

The Consul Go SDK automatically loads the following standard environment variables via `consulapi.DefaultConfig()`:

- `CONSUL_HTTP_ADDR` - Consul server address (default: `127.0.0.1:8500`)
- `CONSUL_HTTP_TOKEN` - Consul ACL token
- `CONSUL_NAMESPACE` - Consul namespace (Enterprise)
- `CONSUL_CACERT` - Path to CA certificate file
- `CONSUL_CLIENT_CERT` - Path to client certificate file
- `CONSUL_CLIENT_KEY` - Path to client private key file
- `CONSUL_HTTP_SSL` - Enable HTTPS (set to `true`)
- `CONSUL_HTTP_SSL_VERIFY` - Enable TLS verification (default: `true`)
- `CONSUL_TLS_SERVER_NAME` - Server name for TLS SNI

**Note on ACL Tokens:** If your Consul cluster has ACLs enabled, you must provide a token with appropriate permissions via the `CONSUL_HTTP_TOKEN` environment variable or `--consul-token` flag. Without a valid token, `consulKey()` and `consulKeys()` functions will fail with permission denied errors.

**Priority:** CLI flags take precedence over environment variables.

##### Configuration Examples

**Basic HTTP connection using CLI flags:**
```bash
nomad-pack run my-pack --consul-address=http://localhost:8500
``` 

**Basic HTTP connection using environment variables:**
```bash
export CONSUL_HTTP_ADDR=http://localhost:8500
nomad-pack run my-pack
``` 

**HTTPS with authentication:**
```bash
nomad-pack run my-pack \
  --consul-address=https://consul.example.com:8501 \
  --consul-token=my-secret-token
```

**HTTPS with TLS verification:**
```bash
nomad-pack run my-pack \
  --consul-address=https://consul.example.com:8501 \
  --consul-token=my-secret-token \
  --consul-ca-cert=/path/to/ca.pem
```

**Mutual TLS (mTLS):**
```bash
nomad-pack run my-pack \
  --consul-address=https://consul.example.com:8501 \
  --consul-ca-cert=/path/to/ca.pem \
  --consul-client-cert=/path/to/client.pem \
  --consul-client-key=/path/to/client-key.pem
```

### Region functions

#### `nomadRegions` <a id="nomadRegions"></a>

##### Parameters

- None


##### Returns

- `error` or `[]string` containing region names known to the cluster.

##### Example

```

```

##### Output

```
```



## Network functions <a id="topicNetwork"></a>

Nomad-pack provides some helper functions that leverage Golang's `netip` package
for IP address parsing and validation.

**NOTE:** The parse functions will error for invalid addresses which will prevent
the template from rendering. Any part of the template that uses `parseAddr` or
`parseAddrPort` should be guarded using the corresponding validation function,
`validAddr` or `validAddrPort`

## Debugging functions <a id="topicDebugging">

### `spewDump` <a id="spewDump"></a>

Returns a string representation of the provided value to the template using
`spew.Sdump`.

> `Sdump` displays the passed parameters to standard out with newlines, customizable
> indentation, and additional debug information such as complete types and all pointer
> addresses used to indirect to the final value. It provides the following features over
> the built-in printing facilities provided by the `fmt` package:
>
> Pointers are dereferenced and followed
> Circular data structures are detected and handled properly
> Custom `Stringer`/`error` interfaces are optionally invoked, including on unexported types
> Custom types which only implement the `Stringer`/`error` interfaces via a pointer receiver
> are optionally invoked when passing non-pointer variables
> Byte arrays and slices are dumped like the `hexdump -C` command which includes offsets, > byte values in hex, and ASCII output

The configuration for the standard Spew printer is as follows:

```go
Indent: " "
MaxDepth: 0
DisableMethods: false
DisablePointerMethods: false
ContinueOnMethod: false
SortKeys: false
```

##### Parameters

- 1: `any` - The object to print via `spew.Sdump`

##### Returns

- a string representation of the object passed as the parameter

##### Example

Dump the current context value for debugging purposes

```
[[ spewDump . ]]
```

### `spewPrintf` <a id="spewPrintf"></a>
Returns a formatted string representation of a value using `spew.Sprintf`.

Returns a new [`spew.ConfigState`][] with default configuration. This will need
to be captured as a variable for reuse. The `customSpew` function is implemented
by `spew.NewDefaultConfig`.

> `NewDefaultConfig` returns a `spew.ConfigState` with the following default settings.
> ```go
>Indent: " "
>MaxDepth: 0
>DisableMethods: false
>DisablePointerMethods: false
>ContinueOnMethod: false
>SortKeys: false
>```

##### Parameters

- None

##### Returns

- A `spew.ConfigState`, which is a customized Sprig printer suitable to be passed as
    an argument to the customizing functions for further settings changes.

##### Example

Change the default indentation from one space to a tab and dump the current
template context in place.

```
[[ $cs := ( customSpew | withIndent "  " ) ]][[ $cs.Sdump . ]]
```


### Custom debug output format functions

The [Spew][] package provides a custom debug output format for Go data structures
to aid in debugging. The following functions are used to create a custom Spew configuration.

#### `customSpew` <a id="customSpew"></a>

Returns a new [`spew.ConfigState`][] with default configuration. This will need
to be captured as a variable for reuse. The `customSpew` function is implemented
by `spew.NewDefaultConfig`.

> `NewDefaultConfig` returns a `spew.ConfigState` with the following default settings.

> ```go
>Indent: " "
>MaxDepth: 0
>DisableMethods: false
>DisablePointerMethods: false
>ContinueOnMethod: false
>SortKeys: false
>```

##### Parameters

- None

##### Returns

- `spew.ConfigState`, which is a customized Sprig printer suitable to be passed as
    an argument to the customizing functions for further settings changes.

##### Example

Change the default indentation from one space to a tab and dump the current
template context in place.

```
[[ $cs := ( customSpew | withIndent "  " ) ]][[ $cs.Sdump . ]]
```

#### `withIndent` <a id="withIndent"></a>

Sets the `Indent` flag for a customized Spew printer. From the Spew documentation:

> `Indent` specifies the string to use for each indentation level.  The
> global config instance that all top-level functions use sets this to a
> single space by default.  If you would like more indentation, you might
> set this to a tab with `"\t"` or perhaps two spaces with `"  "`.

##### Parameters

- 1: `string` - The value to set the `Indent` value of the `s` parameter to.
- 2: `spew.ConfigState` - A customized Sprig printer created by `customSpew`

##### Returns

- The modified Sprig printer

##### Example

Change the default indentation from one space to a tab and dump the current
template context in place.

```
[[ $cs := ( customSpew | withIndent "  " ) ]][[ $cs.Sdump . ]]
```

#### `withMaxDepth` <a id="withMaxDepth"></a>

Sets the `MaxDepth` flag for a customized Spew printer. From the Spew documentation:

> `MaxDepth` controls the maximum number of levels to descend into nested
> data structures.  The default, `0`, means there is no limit.
>
> **NOTE:** Circular data structures are properly detected, so it is not
> necessary to set this value unless you specifically want to limit deeply
> nested data structures.

##### Parameters

- 1: `int` - The value to set the `MaxDepth` value of the `s` parameter to.
- 2: `spew.ConfigState` - A customized Sprig printer created by `customSpew`

##### Returns

- The modified Sprig printer

##### Example

```
[[ $cs := ( customSpew | withMaxDepth true ) ]][[ $cs.Sdump . ]]
```

#### `withDisableMethods` <a id="withDisableMethods"></a>

Sets the `DisableMethods` flag for a customized Spew printer. From the Spew documentation:

> `DisableMethods` specifies whether or not `error` and `Stringer` interfaces are
> invoked for types that implement them.

##### Parameters

- 1: `bool` - The value to set the `DisableMethods` value of the `s` parameter to.
- 2: `spew.ConfigState` - A customized Sprig printer created by `customSpew`

##### Returns

- The modified Sprig printer

##### Example

```
[[ $cs := ( customSpew | withDisableMethods true ) ]][[ $cs.Sdump . ]]
```

#### `withDisablePointerMethods` <a id="withDisablePointerMethods"></a>

Sets the `DisablePointerMethods` flag for a customized Spew printer. From the
Spew documentation:

> `DisablePointerMethods` specifies whether or not to check for and invoke
> `error` and `Stringer` interfaces on types which only accept a pointer
> receiver when the current type is not a pointer.
>
> **NOTE:** This might be an unsafe action since calling one of these methods
> with a pointer receiver could technically mutate the value, however,
> in practice, types which choose to satisfy an `error` or `Stringer`
> interface with a pointer receiver should not be mutating their state
> inside these interface methods.  As a result, this option relies on
> access to the `unsafe` package, so it will not have any effect when
> running in environments without access to the `unsafe` package such as
> Google App Engine or with the "safe" build tag specified.

##### Parameters

- 1: `bool` - The value to set the `DisablePointerMethods` value of the `s` parameter to.
- 2: `spew.ConfigState` - A customized Sprig printer created by `customSpew`

##### Returns

- The modified Sprig printer

##### Example

```
[[ $cs := ( customSpew | withDisablePointerMethods true ) ]][[ $cs.Sdump . ]]
```

#### `withDisablePointerAddresses` <a id="withDisablePointerAddresses"></a>

Sets the `DisablePointerAddresses` flag for a customized Spew printer. From the Spew documentation:

> `DisablePointerAddresses` specifies whether to disable the printing of
> pointer addresses. This is useful when diffing data structures in tests.

##### Parameters

- 1: `bool` - The value to set the `DisablePointerAddresses` value of the `s` parameter to.
- 2: `spew.ConfigState` - A customized Sprig printer created by `customSpew`

##### Returns

- The modified Sprig printer

##### Example

```
[[ $cs := ( customSpew | withDisablePointerAddresses true ) ]][[ $cs.Sdump . ]]
```

#### `withDisableCapacities` <a id="withDisableCapacities"></a>

Sets the `DisableCapacities` flag for a customized Spew printer. From the Spew documentation:

> `DisableCapacities` specifies whether to disable the printing of capacities
> for arrays, slices, maps and channels. This is useful when diffing
> data structures in tests.

##### Parameters

- 1: `bool` - The value to set the `DisableCapacities` value of the `s` parameter to.
- 2: `spew.ConfigState` - A customized Sprig printer created by `customSpew`

##### Returns

- The modified Sprig printer

##### Example

```
[[ $cs := ( customSpew | withDisableCapacities true ) ]][[ $cs.Sdump . ]]
```

#### `withContinueOnMethod` <a id="withContinueOnMethod"></a>

Sets the `ContinueOnMethod` flag for a customized Spew printer. From the Spew documentation:

> `ContinueOnMethod` specifies whether or not recursion should continue once
> a custom error or Stringer interface is invoked.  The default, `false`,
> means it will print the results of invoking the custom `error` or `Stringer`
> interface and return immediately instead of continuing to recurse into
> the internals of the data type.
>
> **NOTE:** This flag does not have any effect if method invocation is disabled
> via the `DisableMethods` or `DisablePointerMethods` options.

##### Parameters

- 1: `bool` - The value to set the `ContinueOnMethod` value of the `s` parameter to.
- 2: `spew.ConfigState` - A customized Sprig printer created by `customSpew`

##### Returns

- The modified Sprig printer

##### Example

```
[[ $cs := ( customSpew | withContinueOnMethod true ) ]][[ $cs.Sdump . ]]
```

#### `withSortKeys` <a id="withSortKeys"></a>

Sets the `SortKeys` flag for a customized Spew printer. From the Spew documentation:

> `SortKeys` specifies map keys should be sorted before being printed. Use
> this to have a more deterministic, diffable output.  Note that only
> native types (`bool`, `int`, `uint`, `floats`, `uintptr`, and `string`) and types
> that support the error or `Stringer` interfaces (if methods are
> enabled) are supported, with other types sorted according to the
> `reflect.Value.String()` output which guarantees display stability.

##### Parameters

- 1: `bool` - The value to set the `SortKeys` value of the `s` parameter to.
- 2: `spew.ConfigState` - A customized Sprig printer created by `customSpew`

##### Returns

- The modified Sprig printer

##### Example

```
[[ $cs := ( customSpew | withSortKeys true ) ]][[ $cs.Sdump . ]]
```

#### `withSpewKeys` <a id="withSpewKeys"></a>

Sets the `SpewKeys` flag for a customized Spew printer. From the Spew documentation:

> `SpewKeys` specifies that, as a last resort attempt, map keys should
> be spewed to strings and sorted by those strings.  This is only
> considered if `SortKeys` is true.

##### Parameters

- 1: `bool` - The value to set the `SpewKeys` value of the `s` parameter to.
- 2: `spew.ConfigState` - A customized Sprig printer created by `customSpew`

Returns:

- The modified Sprig printer

##### Example

```
[[ $cs := ( customSpew | withSpewKeys true ) ]][[ $cs.Sdump . ]]
```

### Helper functions <a id="topicHelpers"></a>

#### `fileContents` <a id="fileContents"></a>

Imports the contents of a file on the local file system into the template at runtime.
The `fileContents` function is run when nomad-pack parses the template.

##### Parameters

- 1: `string` - The path to the file to read.

Returns:

- The contents of the file.

##### Example

**./assets/hello.txt**

```plaintext
hello from file
```

**Template**
```
**[[ fileContents ./assets/hello.txt ]]**
```

**Output**

```
**hello from file**
```

#### `tpl` <a id="tpl"></a>

The `tpl` function renders a template string using the current template context. This is useful for rendering dynamic templates stored in variables or for evaluating template expressions within strings.

The function has access to the same FuncMap and variables as the parent template, including all Sprig functions and Nomad Pack custom functions.

##### Parameters

- 1: `string` - The template string to render
- 2: `interface{}` - The data context to use for rendering (typically `.` to pass the current context)

##### Returns

- `string` - The rendered template output
- `error` - Any error encountered during template parsing or execution

##### Example

Render a template stored in a variable:

```
[[ $tmpl := "Hello, [[ .name ]]!" -]]
[[ tpl $tmpl (dict "name" "World") ]]
```

**Output**

```
Hello, World!
```

Render using pack variables with the `var` function:

```
[[ $greeting := "Deploying [[ var \"job_name\" . ]] to [[ var \"region\" . ]]" -]]
[[ tpl $greeting . ]]
```

**Output**

```
Deploying my-job to us-west-1
```

#### `toStringList` <a id="toStringList"></a>

The `toStringList` function will convert a slice of `any` into an HCL/JSON like
representation by using Go's native Sprintf "%q" formatting.

##### Parameters

- 1: `[]any` - A slice of items to convert into an HCL list.

Returns:

- A string representation of the provided slice

##### Example

```
[[ $cs := ( customSpew | withDisableCapacities true ) ]][[ $cs.Sdump . ]]
```

## Alphabetical list of functions <a id="fnByAlpha></a>

These are the additional functions supplied by Nomad Pack itself.

- [`customSpew`][] - Returns a new `spew.ConfigState` with default configuration; used to build a custom Spew printer.
- [`consulKey`][] - Retrieves a single key-value pair from Consul's KV store.
- [`consulKeys`][] - Retrieves multiple key-value pairs from Consul's KV store with a given prefix.
- [`fileContents`][] - Returns the contents of a file as a string.
- [`nomadNamespace`][] - Returns the current namespace from the Nomad client.
- [`nomadNamespaces`][] - Returns a list of namespaces from the Nomad client.
- [`nomadRegions`][] - Returns a list of regions from the Nomad client.
- [`nomadVariable`][] - Retrieves a specific Nomad Variable by path and namespace.
- [`nomadVariables`][] - Lists all Nomad Variables in the specified namespace.
- [`spewDump`][] - Returns a string representation of a value using `spew.Sdump`.
- [`spewPrintf`][] - Returns a formatted string representation of a value using `spew.Sprintf`.
- [`toStringList`][] - Converts a value to a string list.
- [`tpl`][] - Renders a template string using the current template context.
- [`withContinueOnMethod`][] - Sets the `ContinueOnMethod` flag for a `customSpew`.
- [`withDisableCapacities`][] - Sets the `DisableCapacities` flag for a `customSpew`.
- [`withDisableMethods`][] - Sets the `DisableMethods` flag for a `customSpew`.
- [`withDisablePointerAddresses`][] - Sets the `DisablePointerAddresses` flag for a `customSpew`.
- [`withDisablePointerMethods`][] - Sets the `DisablePointerMethods` flag for a `customSpew`.
- [`withIndent`][] - Sets the indentation level for a `customSpew`.
- [`withMaxDepth`][] - Sets the maximum depth for a `customSpew`.
- [`withSortKeys`][] - Sets the `SortKeys` flag for a `customSpew`.
- [`withSpewKeys`][] - Sets the `SpewKeys` for a `customSpew`.


[Spew]: https://pkg.go.dev/github.com/davecgh/go-spew/spew
[`spew.ConfigState`]: https://pkg.go.dev/github.com/davecgh/go-spew/spew#ConfigState
[Sprig]: https://masterminds.github.io/sprig/
[`*api.Namespace`]: https://developer.hashicorp.com/nomad/api-docs/namespaces#sample-response-1

[fnByAlpha]: #fnByAlpha
[topicNomadAPI]: #topicNomadAPI
[topicNetwork]: #topicNetwork
[topicDebugging]: #topicDebugging
[topicHelpers]: #topicHelpers

[`customSpew`]: #customSpew
[`consulKey`]: #consulKey
[`consulKeys`]: #consulKeys
[`fileContents`]: #fileContents
[`nomadNamespaces`]: #nomadNamespaces
[`nomadNamespace`]: #nomadNamespace
[`nomadRegions`]: #nomadRegions
[`toStringList`]: #toStringList
[`tpl`]: #tpl
[`spewDump`]: #spewDump
[`spewPrintf`]: #spewPrintf
[`withIndent`]: #withIndent
[`withMaxDepth`]: #withMaxDepth
[`withDisableMethods`]: #withDisableMethods
[`withDisablePointerMethods`]: #withDisablePointerMethods
[`withDisablePointerAddresses`]: #withDisablePointerAddresses
[`withDisableCapacities`]: #withDisableCapacities
[`withContinueOnMethod`]: #withContinueOnMethod
[`withSortKeys`]: #withSortKeys
[`withSpewKeys`]: #withSpewKeys
