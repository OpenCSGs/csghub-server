package rebac

import (
	"context"
	"errors"
)

var (
	// ErrDenied indicates that a valid authorization request was not allowed.
	ErrDenied = errors.New("rebac: access denied")
	// ErrInvalidRequest indicates a malformed authorization request.
	ErrInvalidRequest = errors.New("rebac: invalid request")
	// ErrInvalidSubject indicates a malformed or unsupported subject.
	ErrInvalidSubject = errors.New("rebac: invalid subject")
	// ErrInvalidObject indicates a malformed object.
	ErrInvalidObject = errors.New("rebac: invalid object")
	// ErrInvalidRelation indicates a malformed relation.
	ErrInvalidRelation = errors.New("rebac: invalid relation")
	// ErrInvalidSchema indicates an invalid schema definition.
	ErrInvalidSchema = errors.New("rebac: invalid schema")
	// ErrUnsupportedObjectType indicates that the object type is absent from the current schema.
	ErrUnsupportedObjectType = errors.New("rebac: unsupported object type")
	// ErrUnsupportedRelation indicates that the relation is absent from the object's schema.
	ErrUnsupportedRelation = errors.New("rebac: unsupported relation")
	// ErrUnsupported indicates that the Provider does not support a requested capability.
	ErrUnsupported = errors.New("rebac: unsupported capability")
	// ErrProviderUnavailable indicates that the configured Provider cannot currently process a request.
	ErrProviderUnavailable = errors.New("rebac: provider unavailable")
	// ErrProviderTimeout indicates that Provider execution exceeded its deadline.
	ErrProviderTimeout = errors.New("rebac: provider timeout")
	// ErrInvalidProviderResponse indicates that a Provider response violates the public contract.
	ErrInvalidProviderResponse = errors.New("rebac: invalid provider response")
	// ErrBatchLimitExceeded indicates that a batch exceeds the configured maximum size.
	ErrBatchLimitExceeded = errors.New("rebac: batch limit exceeded")
	// ErrDuplicateCorrelationID indicates duplicate correlation IDs in a batch request.
	ErrDuplicateCorrelationID = errors.New("rebac: duplicate correlation ID")
)

// ErrorClass identifies a stable, low-cardinality authorization error category.
type ErrorClass string

const (
	// ErrorClassNone indicates a successful operation.
	ErrorClassNone ErrorClass = "none"
	// ErrorClassDenied indicates that authorization was denied.
	ErrorClassDenied ErrorClass = "denied"
	// ErrorClassInvalid indicates an invalid request, schema, or response format.
	ErrorClassInvalid ErrorClass = "invalid"
	// ErrorClassUnsupported indicates an unsupported type, relation, or capability.
	ErrorClassUnsupported ErrorClass = "unsupported"
	// ErrorClassTimeout indicates that Provider execution timed out.
	ErrorClassTimeout ErrorClass = "timeout"
	// ErrorClassUnavailable indicates that the Provider is unavailable.
	ErrorClassUnavailable ErrorClass = "unavailable"
	// ErrorClassCanceled indicates that the caller canceled the operation.
	ErrorClassCanceled ErrorClass = "canceled"
	// ErrorClassUnknown indicates an unclassified Provider error.
	ErrorClassUnknown ErrorClass = "unknown"
)

// ClassifyError maps an error to a stable observation category.
func ClassifyError(err error) ErrorClass {
	switch {
	case err == nil:
		return ErrorClassNone
	case errors.Is(err, ErrDenied):
		return ErrorClassDenied
	case errors.Is(err, ErrProviderTimeout), errors.Is(err, context.DeadlineExceeded):
		return ErrorClassTimeout
	case errors.Is(err, ErrProviderUnavailable):
		return ErrorClassUnavailable
	case errors.Is(err, context.Canceled):
		return ErrorClassCanceled
	case errors.Is(err, ErrUnsupportedObjectType), errors.Is(err, ErrUnsupportedRelation), errors.Is(err, ErrUnsupported):
		return ErrorClassUnsupported
	case errors.Is(err, ErrInvalidRequest),
		errors.Is(err, ErrInvalidSubject),
		errors.Is(err, ErrInvalidObject),
		errors.Is(err, ErrInvalidRelation),
		errors.Is(err, ErrInvalidSchema),
		errors.Is(err, ErrInvalidProviderResponse),
		errors.Is(err, ErrBatchLimitExceeded),
		errors.Is(err, ErrDuplicateCorrelationID):
		return ErrorClassInvalid
	default:
		return ErrorClassUnknown
	}
}
