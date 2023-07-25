package fixture

import (
	"github.com/hashicorp/nomad-pack/internal/pkg/varfile"
	"github.com/hashicorp/nomad-pack/sdk/pack"
)

const GoodConfigfileHCL = `# variable answers
simple_raw_exec.child1.username="foo"
simple_raw_exec.child1.password="bar"
simple_raw_exec.rootuser="admin"
`

const GoodConfigfileJSON = `{
	"simple_raw_exec.child1.username": "foo",
	"simple_raw_exec.child1.password": "bar",
	"simple_raw_exec.rootuser": "admin"
}`

const BadMissingEqualOneLine = `mypack.foo "bar"`
const BadMissingEqualSecondLine = `mypack.foo = "bar"
bad value`
const BadMissingEqualInternalLine = `mypack.foo = "bar"
bad value
mypack.bar = "baz"`

const BadJSONMissingStartBrace = `"mypack.foo": "bar" }`
const BadJSONMissingEndBrace = `{ "mypack.foo": "bar"`
const BadJSONMissingComma = `{ "mypack.foo": "bar" "mypack.bar": "baz" }`
const BadJSONMissingQuote = `{ "mypack.foo": "bar", mypack.bar": "baz" }`
const BadJSONMissingColon = `{ "mypack.foo": "bar", mypack.bar" "baz" }`
const JSONEmpty = ""
const JSONEmptyObject = "{}"

var JSONFiles = map[varfile.PackID][]*pack.File{
	"myPack": {
		{
			Name:    "tc1.json",
			Content: []byte(BadJSONMissingStartBrace),
			Path:    "/tmp/tc1.json",
		},
		{
			Name:    "tc2.json",
			Content: []byte(BadJSONMissingEndBrace),
			Path:    "/tmp/tc2.json",
		},
	},
}
