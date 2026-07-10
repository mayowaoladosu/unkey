package handler

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
func TestEncodePolicies_GoldenWireFormat(t *testing.T) {
	docs, err := encodePolicies([]openapi.Policy{{
		Id:       ptr.P("pol_golden"),
		Name:     "KEBAP",
		Enabled:  false,
		Firewall: &openapi.FirewallPolicy{Action: "ACTION_DENY"},
	}})
	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Equal(t, "pol_golden", docs[0].ID)

	require.JSONEq(t,
		`{"id":"pol_golden","name":"KEBAP","enabled":false,"firewall":{"action":"ACTION_DENY"}}`,
		string(docs[0].Raw),
	)

	var keys map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(docs[0].Raw, &keys))
	require.Len(t, keys, 4)
	require.NotContains(t, keys, "type")
	require.Contains(t, keys, "enabled")
}

func TestEncodePolicies_EmptyObjectsStayObjects(t *testing.T) {
	openapiCfg := openapi.OpenapiPolicy{}
	docs, err := encodePolicies([]openapi.Policy{
		{
			Id:      ptr.P("pol_1"),
			Name:    "openapi",
			Enabled: true,
			Openapi: &openapiCfg,
		},
		{
			Id:      ptr.P("pol_2"),
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

	require.JSONEq(t, `{"id":"pol_1","name":"openapi","enabled":true,"openapi":{}}`, string(docs[0].Raw))
	require.JSONEq(t,
		`{"id":"pol_2","name":"ratelimit","enabled":true,"ratelimit":{"limit":100,"windowMs":60000,"identifier":{"remoteIp":{}}}}`,
		string(docs[1].Raw),
	)
}

func TestEncodePolicies_GeneratesIdWhenAbsent(t *testing.T) {
	docs, err := encodePolicies([]openapi.Policy{{
		Name:     "no id",
		Enabled:  true,
		Firewall: &openapi.FirewallPolicy{Action: "ACTION_DENY"},
	}})
	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Regexp(t, `^pol_`, docs[0].ID)
	require.Contains(t, string(docs[0].Raw), docs[0].ID)
}

func TestEncodePolicies_FullFeaturedPolicies(t *testing.T) {
	present := openapi.MatchExprHeaderPresent(true)
	_, err := encodePolicies([]openapi.Policy{{
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
	_, err := encodePolicies([]openapi.Policy{{
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

func TestParseStoredPolicies(t *testing.T) {
	empty, err := parseStoredPolicies(nil)
	require.NoError(t, err)
	require.Nil(t, empty)

	legacy, err := parseStoredPolicies([]byte("{}"))
	require.NoError(t, err)
	require.Empty(t, legacy)

	docs, err := parseStoredPolicies([]byte(`{"policies":[{"id":"pol_a"},{"name":"no id"}]}`))
	require.NoError(t, err)
	require.Len(t, docs, 2)
	require.Equal(t, "pol_a", docs[0].ID)
	require.Empty(t, docs[1].ID)
	require.JSONEq(t, `{"name":"no id"}`, string(docs[1].Raw))

	_, err = parseStoredPolicies([]byte(`not json`))
	require.Error(t, err)

	_, err = parseStoredPolicies([]byte(`{"policies":["not an object"]}`))
	require.Error(t, err)
}

// Stored policies the request does not mention must survive a
// read-modify-write byte for byte, including variants this API cannot create
// (jwtauth) and fields newer than our proto gen, which the DiscardUnknown
// parse must tolerate. New documents land after existing ones.
func TestMergePolicies_PassthroughAndAppend(t *testing.T) {
	jwtauth := `{"id":"pol_jwt","name":"jwt","enabled":true,"jwtauth":{}}`
	future := `{"id":"pol_future","fromTheFuture":{"x":1}}`
	stored := []policyDoc{
		{ID: "pol_jwt", Raw: json.RawMessage(jwtauth)},
		{ID: "pol_future", Raw: json.RawMessage(future)},
	}

	incoming, err := encodePolicies([]openapi.Policy{{
		Id:       ptr.P("pol_new"),
		Name:     "deny",
		Enabled:  true,
		Firewall: &openapi.FirewallPolicy{Action: "ACTION_DENY"},
	}})
	require.NoError(t, err)

	blob, err := mergePolicies(stored, incoming, false)
	require.NoError(t, err)
	require.Contains(t, string(blob), jwtauth)
	require.Contains(t, string(blob), future)

	docs, err := parseStoredPolicies(blob)
	require.NoError(t, err)
	require.Len(t, docs, 3)
	require.Equal(t, jwtauth, string(docs[0].Raw))
	require.Equal(t, future, string(docs[1].Raw))
	require.Equal(t, "pol_new", docs[2].ID)
}

func TestMergePolicies_UpdateInPlace(t *testing.T) {
	stored := []policyDoc{
		{ID: "pol_a", Raw: json.RawMessage(`{"id":"pol_a","name":"a","enabled":true,"openapi":{}}`)},
		{ID: "pol_b", Raw: json.RawMessage(`{"id":"pol_b","name":"b","enabled":true,"openapi":{}}`)},
		{ID: "pol_c", Raw: json.RawMessage(`{"id":"pol_c","name":"c","enabled":true,"openapi":{}}`)},
	}

	// Variant change on update is allowed: b turns from openapi into firewall.
	incoming, err := encodePolicies([]openapi.Policy{{
		Id:       ptr.P("pol_b"),
		Name:     "KEBAP",
		Enabled:  false,
		Firewall: &openapi.FirewallPolicy{Action: "ACTION_DENY"},
	}})
	require.NoError(t, err)

	blob, err := mergePolicies(stored, incoming, false)
	require.NoError(t, err)

	docs, err := parseStoredPolicies(blob)
	require.NoError(t, err)
	require.Len(t, docs, 3)
	require.Equal(t, []string{"pol_a", "pol_b", "pol_c"}, []string{docs[0].ID, docs[1].ID, docs[2].ID})
	require.JSONEq(t,
		`{"id":"pol_b","name":"KEBAP","enabled":false,"firewall":{"action":"ACTION_DENY"}}`,
		string(docs[1].Raw),
	)
}

// Prune drops everything the request does not mention (including unknown
// variants) and the result order is exactly the incoming order.
func TestMergePolicies_PruneReplacesAndOrders(t *testing.T) {
	stored := []policyDoc{
		{ID: "pol_a", Raw: json.RawMessage(`{"id":"pol_a","name":"a","enabled":true,"openapi":{}}`)},
		{ID: "pol_jwt", Raw: json.RawMessage(`{"id":"pol_jwt","name":"jwt","enabled":true,"jwtauth":{}}`)},
	}

	incoming, err := encodePolicies([]openapi.Policy{
		{Id: ptr.P("pol_new"), Name: "new first", Enabled: true, Firewall: &openapi.FirewallPolicy{Action: "ACTION_DENY"}},
		{Id: ptr.P("pol_a"), Name: "a moved", Enabled: true, Openapi: &openapi.OpenapiPolicy{}},
	})
	require.NoError(t, err)

	blob, err := mergePolicies(stored, incoming, true)
	require.NoError(t, err)
	require.NotContains(t, string(blob), "pol_jwt")

	docs, err := parseStoredPolicies(blob)
	require.NoError(t, err)
	require.Len(t, docs, 2)
	require.Equal(t, []string{"pol_new", "pol_a"}, []string{docs[0].ID, docs[1].ID})
}

func TestMergePolicies_PruneEmptyClears(t *testing.T) {
	stored := []policyDoc{
		{ID: "pol_a", Raw: json.RawMessage(`{"id":"pol_a","name":"a","enabled":true,"openapi":{}}`)},
	}

	blob, err := mergePolicies(stored, nil, true)
	require.NoError(t, err)
	require.JSONEq(t, `{"policies":[]}`, string(blob))

	docs, err := parseStoredPolicies(blob)
	require.NoError(t, err)
	require.Empty(t, docs)
}
