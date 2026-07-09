package policy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/ptr"
	"github.com/unkeyed/unkey/svc/api/openapi"
)

// The dashboard reads the blob back through a strict schema with a required
// `enabled` boolean and no `type` field, so the emitted JSON must contain
// exactly the fields the request set, nothing more, nothing less.
func TestMarshalPolicies_GoldenWireFormat(t *testing.T) {
	raw, err := MarshalPolicies([]openapi.Policy{{
		Name:     "KEBAP",
		Enabled:  false,
		Firewall: &openapi.FirewallPolicy{Action: "ACTION_DENY"},
	}}, []string{"pol_golden"})
	require.NoError(t, err)
	require.Len(t, raw, 1)

	require.JSONEq(t,
		`{"id":"pol_golden","name":"KEBAP","enabled":false,"firewall":{"action":"ACTION_DENY"}}`,
		string(raw[0]),
	)

	var keys map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw[0], &keys))
	require.Len(t, keys, 4)
	require.NotContains(t, keys, "type")
	require.Contains(t, keys, "enabled")
}

func TestMarshalPolicies_EmptyObjectsStayObjects(t *testing.T) {
	openapiCfg := openapi.OpenapiPolicy{}
	raw, err := MarshalPolicies([]openapi.Policy{
		{
			Name:    "openapi",
			Enabled: true,
			Openapi: &openapiCfg,
		},
		{
			Name:    "ratelimit",
			Enabled: true,
			Ratelimit: &openapi.RatelimitPolicy{
				Limit:      100,
				WindowMs:   60000,
				Identifier: openapi.RatelimitIdentifier{RemoteIp: &map[string]interface{}{}},
			},
		},
	}, []string{"pol_1", "pol_2"})
	require.NoError(t, err)

	require.JSONEq(t, `{"id":"pol_1","name":"openapi","enabled":true,"openapi":{}}`, string(raw[0]))
	require.JSONEq(t,
		`{"id":"pol_2","name":"ratelimit","enabled":true,"ratelimit":{"limit":100,"windowMs":60000,"identifier":{"remoteIp":{}}}}`,
		string(raw[1]),
	)
}

func TestMarshalPolicies_IdCountMismatch(t *testing.T) {
	_, err := MarshalPolicies([]openapi.Policy{{Name: "a", Enabled: true}}, nil)
	require.Error(t, err)
}

func TestAssertWireCompatible_FullFeaturedPolicies(t *testing.T) {
	present := openapi.MatchExprHeaderPresent(true)
	raw, err := MarshalPolicies([]openapi.Policy{{
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
			KeySpaceIds:     []string{"ks_123"},
			PermissionQuery: ptr.P("documents.read"),
			Locations: &[]openapi.KeyLocation{
				{Bearer: &map[string]interface{}{}},
				{Header: &struct {
					Name        string  `json:"name"`
					StripPrefix *string `json:"stripPrefix,omitempty"`
				}{Name: "x-api-key", StripPrefix: ptr.P("Key ")}},
			},
			Ratelimits: &[]openapi.KeyRatelimit{
				{Name: "requests", Limit: ptr.P(int64(100)), Duration: ptr.P(int64(60000)), Cost: ptr.P(int64(2))},
			},
		},
	}}, []string{"pol_full"})
	require.NoError(t, err)

	require.NoError(t, AssertWireCompatible(raw))
}

func TestAssertWireCompatible_RejectsUnknownFields(t *testing.T) {
	err := AssertWireCompatible([]json.RawMessage{
		json.RawMessage(`{"id":"pol_x","name":"x","enabled":true,"bogus":{}}`),
	})
	require.Error(t, err)
}

func TestParseStoredPolicies(t *testing.T) {
	empty, err := ParseStoredPolicies(nil)
	require.NoError(t, err)
	require.Nil(t, empty)

	legacy, err := ParseStoredPolicies([]byte("{}"))
	require.NoError(t, err)
	require.Nil(t, legacy)

	policies, err := ParseStoredPolicies([]byte(`{"policies":[{"id":"a"},{"id":"b"}]}`))
	require.NoError(t, err)
	require.Len(t, policies, 2)

	_, err = ParseStoredPolicies([]byte(`not json`))
	require.Error(t, err)
}

// Stored policies must survive a read-modify-write byte for byte, including
// variants this API cannot create, such as jwtauth.
func TestBuildBlob_PreservesExistingBytesVerbatim(t *testing.T) {
	jwtauth := `{"id":"pol_jwt","name":"jwt","enabled":true,"jwtauth":{}}`
	existing := []json.RawMessage{json.RawMessage(jwtauth)}

	added, err := MarshalPolicies([]openapi.Policy{{
		Name:     "deny",
		Enabled:  true,
		Firewall: &openapi.FirewallPolicy{Action: "ACTION_DENY"},
	}}, []string{"pol_new"})
	require.NoError(t, err)

	blob, err := BuildBlob(existing, added)
	require.NoError(t, err)
	require.Contains(t, string(blob), jwtauth)

	require.NoError(t, AssertParseable(blob))

	roundTripped, err := ParseStoredPolicies(blob)
	require.NoError(t, err)
	require.Len(t, roundTripped, 2)
	require.Equal(t, jwtauth, string(roundTripped[0]))
}
