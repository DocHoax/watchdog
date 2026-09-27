package mcp

import (
	"fmt"
)

// Standard JSON-RPC 2.0 Error Codes
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603

	// Custom Application Error Codes (-32000 to -32099)
	CodeInvalidNodeID    = -32001
	CodeInvalidArgument  = -32002
	CodeUnauthorized     = -32003
	CodeNodeNotFound     = -32004
	CodeForbidden        = -32005
	CodeRateLimited      = -32029
	CodeResourceNotFound = -32004
)

// String Constants for Application Error Identifiers
const (
	ErrCodeStrNodeNotFound     = "NODE_NOT_FOUND"
	ErrCodeStrInvalidNodeID    = "INVALID_NODE_ID"
	ErrCodeStrInvalidArgument  = "INVALID_ARGUMENT"
	ErrCodeStrUnauthorized     = "UNAUTHORIZED"
	ErrCodeStrForbidden        = "FORBIDDEN"
	ErrCodeStrRateLimited      = "RATE_LIMITED"
	ErrCodeStrInternalError    = "INTERNAL_ERROR"
	ErrCodeStrResourceNotFound = "RESOURCE_NOT_FOUND"
)

// NewJSONRPCError creates a new JSONRPCError.
func NewJSONRPCError(code int, message string, data any) *JSONRPCError {
	return &JSONRPCError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// NewParseError returns a -32700 Parse Error.
func NewParseError(details string) *JSONRPCError {
	msg := "Parse error"
	if details != "" {
		msg = fmt.Sprintf("Parse error: %s", details)
	}
	return &JSONRPCError{
		Code:    CodeParseError,
		Message: msg,
	}
}

// NewInvalidRequestError returns a -32600 Invalid Request Error.
func NewInvalidRequestError(details string) *JSONRPCError {
	msg := "Invalid Request"
	if details != "" {
		msg = fmt.Sprintf("Invalid Request: %s", details)
	}
	return &JSONRPCError{
		Code:    CodeInvalidRequest,
		Message: msg,
	}
}

// NewMethodNotFoundError returns a -32601 Method Not Found Error.
func NewMethodNotFoundError(method string) *JSONRPCError {
	return &JSONRPCError{
		Code:    CodeMethodNotFound,
		Message: fmt.Sprintf("Method '%s' not found", method),
	}
}

// NewInvalidParamsError returns a -32602 Invalid Params Error.
func NewInvalidParamsError(details string) *JSONRPCError {
	msg := "Invalid params"
	if details != "" {
		msg = fmt.Sprintf("Invalid params: %s", details)
	}
	return &JSONRPCError{
		Code:    CodeInvalidParams,
		Message: msg,
		Data: map[string]string{
			"error_code": ErrCodeStrInvalidArgument,
		},
	}
}

// NewInternalError returns a -32603 Internal Error.
func NewInternalError(details string) *JSONRPCError {
	msg := "Internal error"
	if details != "" {
		msg = fmt.Sprintf("Internal error: %s", details)
	}
	return &JSONRPCError{
		Code:    CodeInternalError,
		Message: msg,
		Data: map[string]string{
			"error_code": ErrCodeStrInternalError,
		},
	}
}

// NewNodeNotFoundError returns a Node Not Found Error.
func NewNodeNotFoundError(nodeID string) *JSONRPCError {
	return &JSONRPCError{
		Code:    CodeNodeNotFound,
		Message: fmt.Sprintf("Node '%s' not found", nodeID),
		Data: map[string]string{
			"error_code": ErrCodeStrNodeNotFound,
			"node_id":    nodeID,
		},
	}
}

// NewInvalidNodeIDError returns an Invalid Node ID Error.
func NewInvalidNodeIDError(details string) *JSONRPCError {
	msg := "Invalid node ID"
	if details != "" {
		msg = fmt.Sprintf("Invalid node ID: %s", details)
	}
	return &JSONRPCError{
		Code:    CodeInvalidNodeID,
		Message: msg,
		Data: map[string]string{
			"error_code": ErrCodeStrInvalidNodeID,
		},
	}
}

// NewRateLimitedError returns a Rate Limited Error.
func NewRateLimitedError(details string) *JSONRPCError {
	msg := "Rate limit exceeded"
	if details != "" {
		msg = fmt.Sprintf("Rate limit exceeded: %s", details)
	}
	return &JSONRPCError{
		Code:    CodeRateLimited,
		Message: msg,
		Data: map[string]string{
			"error_code": ErrCodeStrRateLimited,
		},
	}
}

// NewUnauthorizedError returns an Unauthorized Error.
func NewUnauthorizedError(details string) *JSONRPCError {
	msg := "Unauthorized"
	if details != "" {
		msg = fmt.Sprintf("Unauthorized: %s", details)
	}
	return &JSONRPCError{
		Code:    CodeUnauthorized,
		Message: msg,
		Data: map[string]string{
			"error_code": ErrCodeStrUnauthorized,
		},
	}
}

// NewForbiddenError returns a Forbidden Error.
func NewForbiddenError(details string) *JSONRPCError {
	msg := "Forbidden"
	if details != "" {
		msg = fmt.Sprintf("Forbidden: %s", details)
	}
	return &JSONRPCError{
		Code:    CodeForbidden,
		Message: msg,
		Data: map[string]string{
			"error_code": ErrCodeStrForbidden,
		},
	}
}

// NewResourceNotFoundError returns a Resource Not Found Error.
func NewResourceNotFoundError(uri string) *JSONRPCError {
	return &JSONRPCError{
		Code:    CodeResourceNotFound,
		Message: fmt.Sprintf("Resource '%s' not found", uri),
		Data: map[string]string{
			"error_code": ErrCodeStrResourceNotFound,
			"uri":        uri,
		},
	}
}
