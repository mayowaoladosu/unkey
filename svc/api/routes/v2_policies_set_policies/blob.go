// The sentinel_config blob is protojson for frontlinev1.Config and is read
// back by the dashboard through a strict schema. Both contracts must hold:
// no unknown fields, required fields present even when zero (enabled=false),
// untouched stored policies never re-serialized.

package handler

import (
	"bytes"
	"encoding/json"
	"fmt"

	frontlinev1 "github.com/unkeyed/unkey/gen/proto/frontline/v1"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/svc/api/openapi"
	"google.golang.org/protobuf/encoding/protojson"
)

type configEnvelope struct {
	Policies []json.RawMessage `json:"policies"`
}

// policyDoc pairs a policy id with its wire JSON so mergePolicies can match
// stored and incoming documents.
type policyDoc struct {
	ID  string
	Raw json.RawMessage
}

// wirePolicy marshals a policy flat with its id. The outer ID shadows the
// embedded Policy's optional id (encoding/json prefers the shallower field).
type wirePolicy struct {
	ID string `json:"id"`
	openapi.Policy
}

// parseStoredPolicies extracts each stored document's id without
// interpreting the rest, so unknown variants such as jwtauth survive a
// read-modify-write byte for byte. Empty and legacy "{}" blobs mean no
// policies, mirroring frontline's ParseMiddleware.
func parseStoredPolicies(raw []byte) ([]policyDoc, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}

	var envelope configEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal stored sentinel config: %w", err)
	}

	docs := make([]policyDoc, 0, len(envelope.Policies))
	for i, p := range envelope.Policies {
		var doc struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(p, &doc); err != nil {
			return nil, fmt.Errorf("unmarshal stored policy %d: %w", i, err)
		}
		docs = append(docs, policyDoc{ID: doc.ID, Raw: p})
	}
	return docs, nil
}

// encodePolicies serializes policies into wire form, keeping a
// client-supplied id (updates that stored policy in place) or generating one
// (never matches stored, lands as an append). The result must strictly parse
// as frontlinev1.Config: a failure is our serialization bug, never a user
// error.
func encodePolicies(policies []openapi.Policy) ([]policyDoc, error) {
	docs := make([]policyDoc, 0, len(policies))
	raws := make([]json.RawMessage, 0, len(policies))
	for _, p := range policies {
		id := uid.New(uid.PolicyPrefix)
		if p.Id != nil {
			id = *p.Id
		}
		raw, err := json.Marshal(wirePolicy{ID: id, Policy: p})
		if err != nil {
			return nil, fmt.Errorf("marshal policy %q: %w", id, err)
		}
		docs = append(docs, policyDoc{ID: id, Raw: raw})
		raws = append(raws, raw)
	}

	blob, err := json.Marshal(configEnvelope{Policies: raws})
	if err != nil {
		return nil, err
	}
	if err := protojson.Unmarshal(blob, &frontlinev1.Config{}); err != nil {
		return nil, fmt.Errorf("encoded policies are not gateway-compatible: %w", err)
	}
	return docs, nil
}

// mergePolicies assembles the blob: stored order kept, id matches replaced
// in place, new documents appended; prune keeps exactly the incoming list.
// The final parse mirrors frontline's ParseMiddleware (DiscardUnknown),
// proving the gateway can load what we are about to store.
func mergePolicies(stored, incoming []policyDoc, prune bool) ([]byte, error) {
	if prune {
		stored = nil
	}

	updates := make(map[string]json.RawMessage, len(incoming))
	for _, doc := range incoming {
		updates[doc.ID] = doc.Raw
	}

	merged := make([]json.RawMessage, 0, len(stored)+len(incoming))
	seen := make(map[string]struct{}, len(stored))
	for _, doc := range stored {
		seen[doc.ID] = struct{}{}
		// An id-less stored document is unaddressable; it always passes through.
		if raw, ok := updates[doc.ID]; ok && doc.ID != "" {
			merged = append(merged, raw)
		} else {
			merged = append(merged, doc.Raw)
		}
	}
	for _, doc := range incoming {
		if _, ok := seen[doc.ID]; !ok {
			merged = append(merged, doc.Raw)
		}
	}

	blob, err := json.Marshal(configEnvelope{Policies: merged})
	if err != nil {
		return nil, err
	}
	gatewayParse := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := gatewayParse.Unmarshal(blob, &frontlinev1.Config{}); err != nil {
		return nil, fmt.Errorf("merged sentinel config is not gateway-parseable: %w", err)
	}
	return blob, nil
}
