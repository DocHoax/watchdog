package mcp

import (
	"testing"
)

func TestJSONRPCErrors(t *testing.T) {
	pe := NewParseError("unexpected end of JSON")
	if pe.Code != CodeParseError || pe.Message != "Parse error: unexpected end of JSON" {
		t.Errorf("NewParseError() unexpected result: %+v", pe)
	}

	ire := NewInvalidRequestError("missing jsonrpc field")
	if ire.Code != CodeInvalidRequest || ire.Message != "Invalid Request: missing jsonrpc field" {
		t.Errorf("NewInvalidRequestError() unexpected result: %+v", ire)
	}

	mne := NewMethodNotFoundError("unknown_method")
	if mne.Code != CodeMethodNotFound || mne.Message != "Method 'unknown_method' not found" {
		t.Errorf("NewMethodNotFoundError() unexpected result: %+v", mne)
	}

	ipe := NewInvalidParamsError("missing arg")
	if ipe.Code != CodeInvalidParams {
		t.Errorf("NewInvalidParamsError() unexpected code: %d", ipe.Code)
	}

	ie := NewInternalError("db timeout")
	if ie.Code != CodeInternalError {
		t.Errorf("NewInternalError() unexpected code: %d", ie.Code)
	}

	nne := NewNodeNotFoundError("node-99")
	if nne.Code != CodeNodeNotFound {
		t.Errorf("NewNodeNotFoundError() unexpected code: %d", nne.Code)
	}

	inide := NewInvalidNodeIDError("bad format")
	if inide.Code != CodeInvalidNodeID {
		t.Errorf("NewInvalidNodeIDError() unexpected code: %d", inide.Code)
	}

	rle := NewRateLimitedError("too fast")
	if rle.Code != CodeRateLimited {
		t.Errorf("NewRateLimitedError() unexpected code: %d", rle.Code)
	}

	ue := NewUnauthorizedError("token missing")
	if ue.Code != CodeUnauthorized {
		t.Errorf("NewUnauthorizedError() unexpected code: %d", ue.Code)
	}

	fe := NewForbiddenError("insufficient permissions")
	if fe.Code != CodeForbidden {
		t.Errorf("NewForbiddenError() unexpected code: %d", fe.Code)
	}

	rne := NewResourceNotFoundError("watchdog://missing")
	if rne.Code != CodeResourceNotFound {
		t.Errorf("NewResourceNotFoundError() unexpected code: %d", rne.Code)
	}
}
