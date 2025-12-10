// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package envloader

import (
	"os"
	"strings"
)

const DefaultPrefix = "NOMAD_PACK_VAR_"

type EnvLoader struct {
	prefix string
}

func New() *EnvLoader {
	return &EnvLoader{prefix: DefaultPrefix}
}

func (e *EnvLoader) GetVarsFromEnv() map[string]string {
	if e.prefix == "" {
		return getVarsFromEnv(DefaultPrefix)
	}
	return getVarsFromEnv(e.prefix)
}

func getVarsFromEnv(prefix string) map[string]string {
	out := make(map[string]string)
	for _, raw := range os.Environ() {
		switch {
		case !strings.HasPrefix(raw, prefix):
			continue
		case !strings.Contains(raw, "="):
			continue
		default:
			key, value, _ := strings.Cut(raw, "=")
			out[strings.TrimPrefix(key, prefix)] = value
		}
	}
	return out
}
