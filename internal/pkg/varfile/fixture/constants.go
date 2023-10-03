package fixture

import (
	"github.com/hashicorp/nomad-pack/sdk/pack"
)

const GoodConfigfileHCL = `# variable answers
child1.username="foo"
child1.password="bar"
rootuser="admin"
`

const GoodConfigfileJSON = `{
	"child1.username": "foo",
	"child1.password": "bar",
	"rootuser": "admin"
}`

const BadMissingEqualOneLine = `foo "bar"`
const BadMissingEqualSecondLine = `foo = "bar"
bad value`
const BadMissingEqualInternalLine = `foo = "bar"
bad value
bar = "baz"`

const BadJSONMissingStartBrace = `"foo": "bar" }`
const BadJSONMissingEndBrace = `{ "foo": "bar"`
const BadJSONMissingComma = `{ "foo": "bar" "bar": "baz" }`
const BadJSONMissingQuote = `{ "foo": "bar", bar": "baz" }`
const BadJSONMissingColon = `{ "foo": "bar", bar" "baz" }`
const JSONEmpty = ""
const JSONEmptyObject = "{}"

var JSONFiles = map[pack.ID][]*pack.File{
	"mypack": {
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
