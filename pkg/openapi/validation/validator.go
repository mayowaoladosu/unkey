package validation

import (
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi-validator/config"
	validatorErrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	"github.com/unkeyed/unkey/pkg/fault"
)

// ValidationError represents a single field-level validation failure.
// Fix is non-nil when the validator can suggest a concrete correction.
type ValidationError struct {
	Message  string
	Location string
	Fix      *string
}

// Result holds the validation outcome when a request is invalid.
// A nil *Result means the request passed validation.
type Result struct {
	Detail string
	Errors []ValidationError
}

// Validator validates HTTP requests against an OpenAPI specification.
type Validator struct {
	validator         validator.Validator
	warmMu            sync.Mutex
	warmedOperations  sync.Map
	requestMediaTypes map[string]struct{}
}

// NewFromBytes creates a Validator from a raw OpenAPI spec.
// Returns an error if the spec cannot be parsed or is itself invalid.
func NewFromBytes(spec []byte) (*Validator, error) {
	document, err := libopenapi.NewDocument(spec)
	if err != nil {
		return nil, fault.Wrap(err, fault.Internal("failed to create OpenAPI document"))
	}

	model, modelErr := document.BuildV3Model()
	if modelErr != nil {
		return nil, fault.New("failed to create validator", fault.Internal(modelErr.Error()))
	}
	v := validator.NewValidatorFromV3Model(&model.Model, config.WithRegexCache(&sync.Map{}))
	v.SetDocument(document)

	valid, docErrors := v.ValidateDocument()
	if !valid {
		messages := make([]fault.Wrapper, len(docErrors))
		for i, e := range docErrors {
			messages[i] = fault.Internal(e.Message)
		}
		return nil, fault.New("openapi document is invalid", messages...)
	}

	requestMediaTypes := map[string]struct{}{"": {}}
	if model.Model.Paths != nil && model.Model.Paths.PathItems != nil {
		for pathPair := model.Model.Paths.PathItems.First(); pathPair != nil; pathPair = pathPair.Next() {
			pathItem := pathPair.Value()
			if pathItem == nil {
				continue
			}
			operations := pathItem.GetOperations()
			if operations == nil {
				continue
			}
			for operationPair := operations.First(); operationPair != nil; operationPair = operationPair.Next() {
				operation := operationPair.Value()
				if operation == nil {
					continue
				}
				requestBody := operation.RequestBody
				if requestBody == nil || requestBody.Content == nil {
					continue
				}
				for contentPair := requestBody.Content.First(); contentPair != nil; contentPair = contentPair.Next() {
					mediaType, _, parseErr := mime.ParseMediaType(contentPair.Key())
					if parseErr == nil {
						requestMediaTypes[strings.ToLower(mediaType)] = struct{}{}
					}
				}
			}
		}
	}

	return &Validator{
		validator:         v,
		requestMediaTypes: requestMediaTypes,
	}, nil
}

// Validate checks r against the OpenAPI spec.
// Returns nil when the request is valid; returns a *Result describing
// the failures otherwise.
func (v *Validator) Validate(r *http.Request) *Result {
	operationKey, cacheable := v.requestOperationKey(r)
	if cacheable {
		if _, warmed := v.warmedOperations.Load(operationKey); warmed {
			return v.validate(r)
		}
	}

	if !cacheable {
		v.warmMu.Lock()
		defer v.warmMu.Unlock()
		return v.validate(r)
	}

	// libopenapi compiles request schemas lazily. Its schema cache is safe for
	// concurrent reads, but concurrent misses can render the same schema at the
	// same time and produce a false circular-reference error. Only first use of
	// an operation/media type is serialized; steady-state validation remains
	// fully concurrent after the compiled schema has entered the shared cache.
	v.warmMu.Lock()
	defer v.warmMu.Unlock()
	if _, warmed := v.warmedOperations.Load(operationKey); warmed {
		return v.validate(r)
	}

	result := v.validate(r)
	if !isSchemaRenderFailure(result) {
		v.warmedOperations.Store(operationKey, struct{}{})
	}
	return result
}

func (v *Validator) validate(r *http.Request) *Result {
	valid, errors := v.validator.ValidateHttpRequestSync(r)

	if !valid {
		errors = filterIgnoredSecurityErrors(errors)
		valid = len(errors) == 0
	}

	if valid {
		return nil
	}

	result := &Result{
		Detail: "One or more fields failed validation",
		Errors: []ValidationError{},
	}

	if len(errors) > 0 {
		err := errors[0]
		result.Detail = err.Message
		for _, verr := range err.SchemaValidationErrors {
			result.Errors = append(result.Errors, ValidationError{
				Message:  verr.Reason,
				Location: verr.KeywordLocation,
				Fix:      nil,
			})
		}
		if len(result.Errors) == 0 {
			howToFix := err.HowToFix
			result.Errors = append(result.Errors, ValidationError{
				Message:  err.Reason,
				Location: err.ValidationType,
				Fix:      &howToFix,
			})
		}
	}

	return result
}

const unsupportedMediaType = "<unsupported>"

func (v *Validator) requestOperationKey(r *http.Request) (string, bool) {
	if r.Pattern == "" {
		// Production requests arrive through http.ServeMux, which sets Pattern.
		// Direct callers have no finite route template, so keep them safe by
		// serializing validation without retaining attacker-controlled URL paths.
		return "", false
	}
	return r.Method + "\x00" + r.Pattern + "\x00" + v.canonicalRequestMediaType(r.Header.Get("Content-Type")), true
}

func (v *Validator) canonicalRequestMediaType(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	contentType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return unsupportedMediaType
	}
	contentType = strings.ToLower(contentType)
	if _, allowed := v.requestMediaTypes[contentType]; allowed {
		return contentType
	}
	if slash := strings.IndexByte(contentType, '/'); slash > 0 {
		wildcard := contentType[:slash] + "/*"
		if _, allowed := v.requestMediaTypes[wildcard]; allowed {
			return wildcard
		}
	}
	if _, allowed := v.requestMediaTypes["*/*"]; allowed {
		return "*/*"
	}
	return unsupportedMediaType
}

func isSchemaRenderFailure(result *Result) bool {
	if result == nil {
		return false
	}
	if strings.Contains(result.Detail, "failed schema rendering") {
		return true
	}
	for _, validationErr := range result.Errors {
		if strings.Contains(validationErr.Message, "schema render failure") {
			return true
		}
	}
	return false
}

// filterIgnoredSecurityErrors drops OpenAPI security-scheme errors that our
// handlers already produce richer messages for. Specifically:
//
//   - "scheme mismatch" (added in libopenapi-validator v0.13): the handler's
//     bearer parser returns a more useful "missing 'Bearer ' prefix" error.
//
// A missing Authorization header is still surfaced by the validator so that
// the existing 400 invalid_input contract is preserved.
func filterIgnoredSecurityErrors(errs []*validatorErrors.ValidationError) []*validatorErrors.ValidationError {
	filtered := make([]*validatorErrors.ValidationError, 0, len(errs))
	for _, e := range errs {
		if e.ValidationType == helpers.SecurityValidation && e.Reason == "Authorization header had incorrect scheme" {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}
