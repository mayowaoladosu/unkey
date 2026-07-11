package validation

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const mediaTypeTestSpec = `
openapi: 3.1.0
info:
  title: media type test
  version: 1.0.0
paths:
  /widgets:
    post:
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
          application/xml:
            schema:
              type: string
      responses:
        "200":
          description: ok
`

func TestRequestOperationKeyBoundsUntrustedValues(t *testing.T) {
	v, err := NewFromBytes([]byte(mediaTypeTestSpec))
	require.NoError(t, err)

	key := func(contentType string) (string, bool) {
		req := httptest.NewRequest("POST", "/widgets", nil)
		req.Pattern = "POST /widgets"
		req.Header.Set("Content-Type", contentType)
		return v.requestOperationKey(req)
	}

	jsonKey, cacheable := key("application/json; charset=utf-8")
	require.True(t, cacheable)
	bareJSONKey, _ := key("application/json")
	require.Equal(t, bareJSONKey, jsonKey)

	xmlKey, _ := key("application/xml")
	require.NotEqual(t, jsonKey, xmlKey)

	unknownA, _ := key("application/x-attacker-a")
	unknownB, _ := key("application/x-attacker-b")
	require.Equal(t, unknownA, unknownB)

	malformedA, _ := key("malformed-a;")
	malformedB, _ := key("malformed-b;")
	require.Equal(t, malformedA, malformedB)
	require.Equal(t, unknownA, malformedA)
}

func TestRequestOperationKeyDoesNotCacheUnmatchedPaths(t *testing.T) {
	v, err := NewFromBytes([]byte(mediaTypeTestSpec))
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/widgets/attacker-controlled", nil)
	req.Header.Set("Content-Type", "application/json")

	key, cacheable := v.requestOperationKey(req)
	require.Empty(t, key)
	require.False(t, cacheable)
}
