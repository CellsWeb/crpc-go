package rpc

import (
	"encoding/json"
	"fmt"

	"github.com/CellsWeb/crpc-go/pkg/schema"
)

// Standard Error Codes matching the CellsWeb RPC protocol
const (
	ErrValidation    = "validation_error"
	ErrUnauthorized  = "unauthorized"
	ErrForbidden     = "forbidden"
	ErrNotFound      = "not_found"
	ErrInvalidBody   = "invalid_body"
	ErrPayloadTooLarge = "payload_too_large"
	ErrConflict        = "conflict"
	ErrRateLimited     = "rate_limited"
	ErrTimeout         = "timeout"
	ErrInternal        = "internal_error"
)

// ValidationErrorDetail alias to schema.ValidationErrorDetail
type ValidationErrorDetail = schema.ValidationErrorDetail

// AppError represents a uniform business, validation, or operational RPC error
type AppError struct {
	Code    string                  `json:"code"`
	Message string                  `json:"message"`
	Details []ValidationErrorDetail `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	if len(e.Details) > 0 {
		return fmt.Sprintf("[%s] %s (%d field errors)", e.Code, e.Message, len(e.Details))
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewError creates a new AppError with code and message
func NewError(code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// NewValidationError creates an AppError with field-level schema validation details
func NewValidationError(message string, details []ValidationErrorDetail) *AppError {
	return &AppError{
		Code:    ErrValidation,
		Message: message,
		Details: details,
	}
}

// ResponseMeta contains request lifecycle tracing metadata
type ResponseMeta struct {
	RequestID  string  `json:"requestId"`
	DurationMs float64 `json:"durationMs"`
}

// ResponseEnvelope represents the uniform HTTP 200 JSON envelope
type ResponseEnvelope struct {
	Status  string                  `json:"status"`            // "ok" or "error"
	Data    any                     `json:"data,omitempty"`    // Present on status == "ok"
	Code    string                  `json:"code,omitempty"`    // Present on status == "error"
	Message string                  `json:"message,omitempty"` // Present on status == "error"
	Details []ValidationErrorDetail `json:"details,omitempty"` // Present on status == "error"
	Meta    ResponseMeta            `json:"meta"`
}

// ToJSON serializes the envelope into bytes
func (r *ResponseEnvelope) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
