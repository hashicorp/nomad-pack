// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package terminal

import (
	"context"
	"sync"

	"github.com/mitchellh/go-glint"
)

// glintLiveView implements LiveView using glint for in-place rendering.
type glintLiveView struct {
	mu        sync.Mutex
	closed    bool
	component glint.Component
}

// newGlintLiveView creates a new live view component.
func newGlintLiveView() *glintLiveView {
	return &glintLiveView{}
}

// SetComponent updates the displayed component layout.
func (v *glintLiveView) SetComponent(c glint.Component) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.component = c
}

// Close marks the view as finalized.
func (v *glintLiveView) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.closed = true
	return nil
}

// Body implements glint.Component. This is called by glint on each render frame.
func (v *glintLiveView) Body(context.Context) glint.Component {
	v.mu.Lock()
	defer v.mu.Unlock()

	var c glint.Component

	if v.component != nil {
		c = v.component
	} else {
		// Empty content - return nothing
		c = glint.Text("")
	}

	// If closed, finalize the content so it becomes permanent
	if v.closed {
		c = glint.Finalize(c)
	}

	return c
}
