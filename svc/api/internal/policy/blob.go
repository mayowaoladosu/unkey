// Package policy validates gateway policies and encodes them into the
// sentinel_config blob stored on app_runtime_settings. The blob is protojson
// for frontlinev1.Config and is also read back by the dashboard through a
// strict schema, so the JSON written here must stay inside both contracts:
// no unknown fields, required fields always present (including enabled=false),
// and existing stored policies are never re-serialized.
package policy

import (
	"bytes"
	"encoding/json"
	"fmt"

	frontlinev1 "github.com/unkeyed/unkey/gen/proto/frontline/v1"
	"github.com/unkeyed/unkey/svc/api/openapi"
	"google.golang.org/protobuf/encoding/protojson"
)

// ParseStoredPolicies returns the raw policy documents from a stored
// sentinel_config blob without interpreting them, so unknown variants such as
// jwtauth survive a read-modify-write byte for byte. An empty blob and the
// legacy "{}" value both mean no policies, mirroring frontline's
// ParseMiddleware.
func ParseStoredPolicies(raw []byte) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("{}")) {
		return nil, nil
	}

	var envelope configEnvelope
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal stored sentinel config: %w", err)
	}
	return envelope.Policies, nil
}

// MarshalPolicies serializes request policies into their stored wire form,
// pairing each with its server-generated id.
func MarshalPolicies(policies []openapi.Policy, ids []string) ([]json.RawMessage, error) {
	if len(policies) != len(ids) {
		return nil, fmt.Errorf("got %d policies but %d ids", len(policies), len(ids))
	}

	out := make([]json.RawMessage, 0, len(policies))
	for i, p := range policies {
		raw, err := json.Marshal(wirePolicy{ID: ids[i], Policy: p})
		if err != nil {
			return nil, fmt.Errorf("marshal policy %q: %w", ids[i], err)
		}
		out = append(out, raw)
	}
	return out, nil
}

// BuildBlob assembles the full sentinel_config blob, appending the added
// policies after the existing ones. Existing raw documents are embedded
// verbatim.
func BuildBlob(existing, added []json.RawMessage) ([]byte, error) {
	merged := make([]json.RawMessage, 0, len(existing)+len(added))
	merged = append(merged, existing...)
	merged = append(merged, added...)
	return json.Marshal(configEnvelope{Policies: merged})
}

// AssertWireCompatible strictly decodes the added policies as
// frontlinev1.Config. A failure means our serialization produced something
// the gateway's proto does not understand, which is a bug on our side.
func AssertWireCompatible(added []json.RawMessage) error {
	blob, err := json.Marshal(configEnvelope{Policies: added})
	if err != nil {
		return err
	}
	return protojson.Unmarshal(blob, &frontlinev1.Config{})
}

// AssertParseable decodes the final blob the way frontline's ParseMiddleware
// does (unknown fields tolerated). A failure means we are about to store a
// config the gateway cannot load.
func AssertParseable(blob []byte) error {
	opts := protojson.UnmarshalOptions{DiscardUnknown: true}
	return opts.Unmarshal(blob, &frontlinev1.Config{})
}

type configEnvelope struct {
	Policies []json.RawMessage `json:"policies"`
}

// wirePolicy is the stored form of a policy: the request body plus the
// server-generated id, inlined via embedding so it marshals flat.
type wirePolicy struct {
	ID string `json:"id"`
	openapi.Policy
}
