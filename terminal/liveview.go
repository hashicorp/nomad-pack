// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package terminal

import "github.com/mitchellh/go-glint"

// LiveView is a component that displays content that updates in-place.
// For glint-based UIs, this renders in-place with each update replacing
// the previous content. For non-interactive UIs, this is a no-op.
type LiveView interface {
	// SetComponents updates the content to display as a glint component layout.
	// This allows for styled/formatted content that updates in-place.
	// Only works with glint-based UIs. For non-interactive UIs, this is a no-op.
	SetComponents(c ...glint.Component)

	// Close marks this view as finalized. The current content becomes
	// permanent and will no longer update.
	Close() error
}
