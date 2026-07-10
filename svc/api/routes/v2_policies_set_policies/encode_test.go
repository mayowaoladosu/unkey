package handler

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/ptr"
	"github.com/unkeyed/unkey/svc/api/openapi"
)

// The dashboard reads the blob back through a strict schema with a required
// `enabled` boolean and no `type` field, so the emitted JSON must contain
// exactly the fields the request set, nothing more, nothing less.
func TestEncodePolicies_GoldenWireFormat(t *testing.T) {
	blob, ids, err := encodePolicies([]openapi.Policy{{
		Name:     "KEBAP",
		Enabled:  false,
		Firewall: &openapi.FirewallPolicy{Action: "ACTION_DENY"},
	}})
	require.NoError(t, err)
	require.Len(t, ids, 1)
	require.NotEmpty(t, ids[0])

	require.JSONEq(t,
		fmt.Sprintf(`{"policies":[{"id":"%s","name":"KEBAP","enabled":false,"firewall":{"action":"ACTION_DENY"}}]}`, ids[0]),
		string(blob),
	)
}

func TestEncodePolicies_EmptyObjectsStayObjects(t *testing.T) {
	blob, ids, err := encodePolicies([]openapi.Policy{
		{
			Name:    "openapi",
			Enabled: true,
			Openapi: &openapi.OpenapiPolicy{},
		},
		{
			Name:    "ratelimit",
			Enabled: true,
			Ratelimit: &openapi.RatelimitPolicy{
				Limit:      100,
				WindowMs:   60000,
				Identifier: openapi.RatelimitIdentifier{RemoteIp: &map[string]any{}},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, ids, 2)

	var envelope struct {
		Policies []json.RawMessage `json:"policies"`
	}
	require.NoError(t, json.Unmarshal(blob, &envelope))
	require.JSONEq(t,
		fmt.Sprintf(`{"id":"%s","name":"openapi","enabled":true,"openapi":{}}`, ids[0]),
		string(envelope.Policies[0]),
	)
	require.JSONEq(t,
		fmt.Sprintf(`{"id":"%s","name":"ratelimit","enabled":true,"ratelimit":{"limit":100,"windowMs":60000,"identifier":{"remoteIp":{}}}}`, ids[1]),
		string(envelope.Policies[1]),
	)
}

func TestEncodePolicies_EmptyRequestClears(t *testing.T) {
	blob, ids, err := encodePolicies(nil)
	require.NoError(t, err)
	require.Empty(t, ids)
	require.JSONEq(t, `{"policies":[]}`, string(blob))
}

// The public request field `keyspaces` must land in the blob under the
// proto's `keySpaceIds` name, or the gateway and dashboard cannot read it.
func TestEncodePolicies_KeyauthWireRename(t *testing.T) {
	blob, ids, err := encodePolicies([]openapi.Policy{{
		Name:    "keyauth",
		Enabled: true,
		Keyauth: &openapi.KeyauthPolicy{Keyspaces: []string{"ks_KEBAP"}},
	}})
	require.NoError(t, err)
	require.JSONEq(t,
		fmt.Sprintf(`{"policies":[{"id":"%s","name":"keyauth","enabled":true,"keyauth":{"keySpaceIds":["ks_KEBAP"]}}]}`, ids[0]),
		string(blob),
	)
	require.NotContains(t, string(blob), `"keyspaces"`)
}

func TestEncodePolicies_FullFeaturedPolicies(t *testing.T) {
	present := openapi.MatchExprHeaderPresent(true)
	_, _, err := encodePolicies([]openapi.Policy{{
		Name:    "keyauth",
		Enabled: true,
		Match: &[]openapi.MatchExpr{
			{Path: &struct {
				Path openapi.StringMatch `json:"path"`
			}{Path: openapi.StringMatch{Prefix: ptr.P("/api/"), IgnoreCase: ptr.P(true)}}},
			{Method: &struct {
				Methods []openapi.MatchExprMethodMethods `json:"methods"`
			}{Methods: []openapi.MatchExprMethodMethods{"GET", "POST"}}},
			{Header: &struct {
				Name    string                          `json:"name"`
				Present *openapi.MatchExprHeaderPresent `json:"present,omitempty"`
				Value   *openapi.StringMatch            `json:"value,omitempty"`
			}{Name: "x-kebap", Present: &present}},
		},
		Keyauth: &openapi.KeyauthPolicy{
			Keyspaces:       []string{"ks_123"},
			PermissionQuery: ptr.P("documents.read"),
			Locations: &[]openapi.KeyLocation{
				{Bearer: &map[string]any{}},
				{Header: &struct {
					Name        string  `json:"name"`
					StripPrefix *string `json:"stripPrefix,omitempty"`
				}{Name: "x-api-key", StripPrefix: ptr.P("Key ")}},
			},
			Ratelimits: &[]openapi.KeyRatelimit{
				{Name: "requests", Limit: ptr.P(int64(100)), Duration: ptr.P(int64(60000)), Cost: ptr.P(int64(2))},
			},
		},
	}})
	require.NoError(t, err)
}

// The strict decode inside encodePolicies must reject JSON the gateway proto
// does not know. The open map on remoteIp is the one place a caller can
// smuggle an unknown field through the generated types.
func TestEncodePolicies_RejectsGatewayUnknownFields(t *testing.T) {
	_, _, err := encodePolicies([]openapi.Policy{{
		Name:    "r",
		Enabled: true,
		Ratelimit: &openapi.RatelimitPolicy{
			Limit:      10,
			WindowMs:   1000,
			Identifier: openapi.RatelimitIdentifier{RemoteIp: &map[string]any{"bogus": true}},
		},
	}})
	require.Error(t, err)
}
