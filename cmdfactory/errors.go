// SPDX-License-Identifier: MIT
// Copyright (c) 2019 GitHub Inc.
// Copyright (c) 2022 Unikraft GmbH.
package cmdfactory

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2/terminal"
)

// FlagErrorf returns a new FlagError that wraps an error produced by
// fmt.Errorf(format, args...).
func FlagErrorf(format string, args ...interface{}) error {
	return FlagErrorWrap(fmt.Errorf(format, args...))
}

// FlagError returns a new FlagError that wraps the specified error.
func FlagErrorWrap(err error) error { return &FlagError{err} }

// A *FlagError indicates an error processing command-line flags or other
// arguments.  Such errors cause the application to display the usage message.
type FlagError struct {
	// Note: not struct{error}: only *FlagError should satisfy error.
	err error
}

func (fe *FlagError) Error() string {
	return fe.err.Error()
}

func (fe *FlagError) Unwrap() error {
	return fe.err
}

// ErrSilent is an error that triggers exit code 1 without any error messaging
var ErrSilent = errors.New("ErrSilent")

// ErrCancel signals user-initiated cancellation
var ErrCancel = errors.New("ErrCancel")

func IsUserCancellation(err error) bool {
	return errors.Is(err, ErrCancel) || errors.Is(err, terminal.InterruptErr)
}

func MutuallyExclusive(message string, conditions ...bool) error {
	numTrue := 0
	for _, ok := range conditions {
		if ok {
			numTrue++
		}
	}
	if numTrue > 1 {
		return FlagErrorf("%s", message)
	}
	return nil
}
