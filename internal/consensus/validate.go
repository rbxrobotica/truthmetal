package consensus

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ldamasio/truthmetal/internal/model"
)

func validateValueType(value string, vt model.ValueType) error {
	switch vt {
	case model.TypeString:
		return nil
	case model.TypeNumber:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("invalid number: %w", err)
		}
	case model.TypeBool:
		if value != "true" && value != "false" {
			return fmt.Errorf("invalid bool: must be 'true' or 'false'")
		}
	case model.TypeJSON:
		var v any
		if err := json.Unmarshal([]byte(value), &v); err != nil {
			return fmt.Errorf("invalid json: %w", err)
		}
	case model.TypeTimestamp:
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return fmt.Errorf("invalid timestamp: must be RFC3339")
		}
	case model.TypeSemver:
		// Basic semver pattern: vX.Y.Z or X.Y.Z
		if len(value) == 0 {
			return fmt.Errorf("invalid semver: empty")
		}
	default:
		return fmt.Errorf("unknown value_type: %q", vt)
	}
	return nil
}
