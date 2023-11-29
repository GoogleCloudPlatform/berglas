// Copyright 2023 The Berglas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logging

import (
	"context"
	"log/slog"
)

// LevelableHandler is an interface which defines a handler that is able to
// dynamically set its level.
type LevelableHandler interface {
	// SetLevel dynamically sets the level on the handler.
	SetLevel(level slog.Level)
}

// Ensure we are a slog handler.
var _ slog.Handler = (*LevelHandler)(nil)

// Ensure we are a levelable handler.
var _ LevelableHandler = (*LevelHandler)(nil)

// LevelHandler is a wrapper around a LevelHandler that gives us the ability to configure
// level at runtime without users needing to manage a separate LevelVar.
type LevelHandler struct {
	handler  slog.Handler
	levelVar *slog.LevelVar
}

// NewLevelHandler creates a new handler that is capable of dynamically setting
// a level in a concurrency-safe way.
func NewLevelHandler(leveler slog.Leveler, h slog.Handler) *LevelHandler {
	if lh, ok := h.(*LevelHandler); ok {
		h = lh.Handler()
	}

	// Ensure what we got is a LeverVar. We also don't want someone giving us a
	// LevelVar and managing it outside of our lifecycle.
	levelVar := new(slog.LevelVar)
	levelVar.Set(leveler.Level())

	return &LevelHandler{
		handler:  h,
		levelVar: levelVar,
	}
}

// SetLevel implements the levelable interface. It adjusts the level of the
// logger. It is safe for concurrent use.
func (h *LevelHandler) SetLevel(level slog.Level) {
	h.levelVar.Set(level)
}

// Enabled implements Handler.Enabled by reporting whether level is at least as
// large as h's level.
func (h *LevelHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.levelVar.Level()
}

// Handle implements Handler.Handle.
func (h *LevelHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.handler.Handle(ctx, r) //nolint:wrapcheck // Want passthrough
}

// WithAttrs implements Handler.WithAttrs.
func (h *LevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return NewLevelHandler(h.levelVar, h.handler.WithAttrs(attrs))
}

// WithGroup implements Handler.WithGroup.
func (h *LevelHandler) WithGroup(name string) slog.Handler {
	return NewLevelHandler(h.levelVar, h.handler.WithGroup(name))
}

// Handler returns the Handler wrapped by h.
func (h *LevelHandler) Handler() slog.Handler {
	return h.handler
}
