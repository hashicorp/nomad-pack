package errors

import (
	stdErrors "errors"
)

var (
	ErrCachePathRequired       = stdErrors.New("cache path is required")
	ErrInvalidCachePath        = stdErrors.New("invalid cache path")
	ErrInvalidRegistryRevision = stdErrors.New("invalid revision")
	ErrInvalidRegistrySource   = stdErrors.New("invalid registry source")
	ErrNoRegistriesAdded       = stdErrors.New("no registries were added to the cache")
	ErrPackNameRequired        = stdErrors.New("pack name is required")
	ErrPackNotFound            = stdErrors.New("pack not found")
	ErrRegistryNameRequired    = stdErrors.New("registry name is required")
	ErrRegistryNotFound        = stdErrors.New("registry not found")
	ErrRegistrySourceRequired  = stdErrors.New("registry source is required")
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
