// +kubebuilder:object:generate=false
package errors

import (
	"errors"
	"fmt"
	"strings"
)

var _ ReasonableError = &ErrorWithReason{}

// ReasonableError enhances an error with a reason.
// The reason is meant to be a CamelCased, machine-readable, enum-like string.
// Use WithReason(err, reason) to wrap a normal error into an ReasonableError.
type ReasonableError interface {
	error
	Reason() string
}

// ErrorWithReason wraps an error and adds a reason to it.
// The reason is meant to be a CamelCased, machine-readable, enum-like string.
// Use WithReason(err, reason) to wrap a normal error into an *ErrorWithReason.
type ErrorWithReason struct {
	error
	reason string
}

// Reason returns the reason for this error.
func (e *ErrorWithReason) Reason() string {
	return e.reason
}

// WithReason wraps an error together with a reason into ErrorWithReason.
// The reason is meant to be a CamelCased, machine-readable, enum-like string.
// If the given error is nil, nil is returned.
func WithReason(err error, reason string) ReasonableError {
	if err == nil {
		return nil
	}
	return &ErrorWithReason{
		error:  err,
		reason: reason,
	}
}

// Errorf works similarly to fmt.Errorf, with the exception that it requires an ErrorWithReason as second argument and returns nil if that one is nil.
// Otherwise, it calls fmt.Errorf to construct an error and wraps it in an ErrorWithReason, using the reason from the given error.
// This is useful for expanding the error message without losing the reason.
func Errorf(format string, err ReasonableError, a ...any) ReasonableError {
	if err == nil {
		return nil
	}
	return WithReason(fmt.Errorf(format, a...), err.Reason())
}

// Join joins multiple errors into a single one.
// Returns nil if all given errors are nil.
// This is equivalent to NewErrorList(errs...).Aggregate().
func Join(errs ...error) ReasonableError {
	return NewReasonableErrorList(errs...).Aggregate()
}

// ReasonableErrorList is a helper struct for situations in which multiple errors (with or without reasons) should be returned as a single one.
type ReasonableErrorList struct {
	Errs    []error
	Reasons []string
}

// NewReasonableErrorList creates a new *ErrorListWithReasons containing the provided errors.
func NewReasonableErrorList(errs ...error) *ReasonableErrorList {
	res := &ReasonableErrorList{
		Errs:    []error{},
		Reasons: []string{},
	}
	return res.Append(errs...)
}

// Aggregate aggregates all errors in the list into a single ErrorWithReason.
// Returns nil if the list is either nil or empty.
// If the list contains a single error, that error is returned.
// Otherwise, a new error is constructed by appending all contained errors' messages.
// The reason in the returned error is the first reason that was added to the list,
// or the empty string if none of the contained errors was an ErrorWithReason.
func (el *ReasonableErrorList) Aggregate() ReasonableError {
	if el == nil || len(el.Errs) == 0 {
		return nil
	}
	reason := ""
	if len(el.Reasons) > 0 {
		reason = el.Reasons[0]
	}
	if len(el.Errs) == 1 {
		if ewr, ok := el.Errs[0].(ReasonableError); ok {
			return ewr
		}
		return WithReason(el.Errs[0], reason)
	}
	sb := strings.Builder{}
	sb.WriteString("multiple errors occurred:")
	for _, e := range el.Errs {
		sb.WriteString("\n")
		sb.WriteString(e.Error())
	}
	return WithReason(errors.New(sb.String()), reason)
}

// Append appends all given errors to the ErrorListWithReasons.
// This modifies the receiver object.
// If a given error is of type ErrorWithReason, its reason is added to the list of reasons.
// nil pointers in the arguments are ignored.
// Returns the receiver for chaining.
func (el *ReasonableErrorList) Append(errs ...error) *ReasonableErrorList {
	for _, e := range errs {
		if e != nil {
			el.Errs = append(el.Errs, e)
			if ewr, ok := e.(ReasonableError); ok {
				el.Reasons = append(el.Reasons, ewr.Reason())
			}
		}
	}
	return el
}

// Reason returns the first reason from the list of reasons contained in this error list.
// If the list is nil or no reasons are contained, the empty string is returned.
// This is equivalent to el.Aggregate().Reason(), except that it also works for an empty error list.
func (el *ReasonableErrorList) Reason() string {
	if el == nil || len(el.Reasons) == 0 {
		return ""
	}
	return el.Reasons[0]
}
