package application

import "errors"

// ErrInvalidInput indicates invalid use-case input.
var ErrInvalidInput = errors.New("invalid input")

// ErrNotFound indicates a requested tenant-scoped resource was not found.
var ErrNotFound = errors.New("not found")

// ErrNotImplemented indicates a dependency is intentionally unavailable in the current stage.
var ErrNotImplemented = errors.New("not implemented")

// ErrInvalidCredentials indicates that a login identity cannot be authenticated.
var ErrInvalidCredentials = errors.New("invalid credentials")
