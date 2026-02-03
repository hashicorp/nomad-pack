// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package errors

var (
	ErrCachePathRequired       = newError("caching path is required")
	ErrInvalidCachePath        = newError("invalid caching path")
	ErrInvalidRegistryRevision = newError("invalid revision")
	ErrInvalidRegistrySource   = newError("invalid registry source")
	ErrNoRegistriesAdded       = newError("no registries were added to the caching")
	ErrPackNameRequired        = newError("pack name is required")
	ErrPackNotFound            = newError("pack not found")
	ErrRegistryNameRequired    = newError("registry name is required")
	ErrRegistryNotFound        = newError("registry not found")
	ErrRegistrySourceRequired  = newError("registry source is required")
)

// UIContextPrefix* are the prefixes commonly used to create a string used in
// UI errors outputs. If a prefix is used more than once, it should have a
// const created.
const (
	RegistryContextPrefixCachePath      = "Cache Path: "
	RegistryContextPrefixRegistrySource = "Registry Source: "
	RegistryContextPrefixRegistryName   = "Registry Name: "
	RegistryContextPrefixPackName       = "Pack Name: "
	RegistryContextPrefixRef            = "Ref: "
)
