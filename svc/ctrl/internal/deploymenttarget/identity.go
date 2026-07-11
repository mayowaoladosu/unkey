// Package deploymenttarget defines stable identities for mutable deployment
// pointers and their append-only assignment records.
package deploymenttarget

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
)

type Kind string

const (
	KindBranch      Kind = "branch"
	KindEnvironment Kind = "environment"
	KindLive        Kind = "live"
)

type Identity struct {
	AppID         string
	EnvironmentID string
	Kind          Kind
	Key           string
}

func ID(identity Identity) (string, error) {
	if identity.AppID == "" || identity.EnvironmentID == "" || identity.Key == "" {
		return "", fmt.Errorf("deployment target identity is incomplete")
	}
	switch identity.Kind {
	case KindBranch, KindEnvironment, KindLive:
	default:
		return "", fmt.Errorf("unsupported deployment target kind %q", identity.Kind)
	}
	return stableID("target", identity.AppID, identity.EnvironmentID, string(identity.Kind), identity.Key), nil
}

func AssignmentID(targetID, operationID string) (string, error) {
	if targetID == "" || operationID == "" {
		return "", fmt.Errorf("deployment target assignment identity is incomplete")
	}
	return stableID("assignment", targetID, operationID), nil
}

func RouteID(fullyQualifiedDomainName string) (string, error) {
	if fullyQualifiedDomainName == "" {
		return "", fmt.Errorf("frontline route identity is incomplete")
	}
	return stableID("route", fullyQualifiedDomainName), nil
}

func stableID(prefix string, values ...string) string {
	digest := sha256.New()
	for _, value := range values {
		writePart(digest, value)
	}
	return prefix + "_" + hex.EncodeToString(digest.Sum(nil)[:16])
}

func writePart(digest hash.Hash, value string) {
	_, _ = fmt.Fprintf(digest, "%d:", len(value))
	_, _ = digest.Write([]byte(value))
}
