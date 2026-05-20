// Package data contains static assets bundled into the binary at compile time.
package data

import _ "embed"

// ReferenceTypesJSON is the bundled JSON array of the 21 DSV reference type
// descriptors. The data is sourced from the upstream /reference-types discovery
// endpoint and bundled rather than fetched at runtime so the server can start
// without network access. See internal/domain/reference.go for the Go constants
// and UPSTREAM.md "Reference Types" for the full documentation.
//
//go:embed reference_types.json
var ReferenceTypesJSON []byte
