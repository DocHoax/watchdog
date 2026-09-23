package reporting

import (
	"encoding/json"
)

// GenerateJSON serializes any data structure into indented, readable JSON.
func GenerateJSON(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
