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
- [`fileContents`][] - Returns the contents of a file as a string.
- [`nomadNamespace`][] - Returns the current namespace from the Nomad client.
- [`nomadNamespaces`][] - Returns a list of namespaces from the Nomad client.
- [`nomadRegions`][] - Returns a list of regions from the Nomad client.
- [`spewDump`][] - Returns a string representation of a value using `spew.Sdump`.
- [`spewPrintf`][] - Returns a formatted string representation of a value using `spew.Sprintf`.
- [`toStringList`][] - Converts a value to a string list.
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
[`fileContents`]: #fileContents
[`nomadNamespaces`]: #nomadNamespaces
[`nomadNamespace`]: #nomadNamespace
[`nomadRegions`]: #nomadRegions
[`toStringList`]: #toStringList
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
