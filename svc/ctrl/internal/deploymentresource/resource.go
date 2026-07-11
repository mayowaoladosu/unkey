// Package deploymentresource maps immutable manifest outputs to stable
// materialized resource identities.
package deploymentresource

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$`)

func ID(deploymentID, outputName string) (string, error) {
	if deploymentID == "" || !validName.MatchString(outputName) {
		return "", fmt.Errorf("deployment resource identity is invalid")
	}
	digest := sha256.Sum256([]byte(deploymentID + ":" + outputName))
	return "resource_" + hex.EncodeToString(digest[:16]), nil
}

func K8sName(deploymentK8sName, outputName string, primary bool) (string, error) {
	if deploymentK8sName == "" || !validName.MatchString(outputName) {
		return "", fmt.Errorf("deployment resource k8s identity is invalid")
	}
	if primary {
		return deploymentK8sName, nil
	}
	base := strings.Trim(strings.ToLower(strings.ReplaceAll(outputName, "_", "-")), "-")
	if base == "" {
		return "", fmt.Errorf("deployment resource name cannot form a k8s name")
	}
	digest := sha256.Sum256([]byte(outputName))
	suffix := hex.EncodeToString(digest[:4])
	maxBase := 63 - len(deploymentK8sName) - len(suffix) - 2
	if maxBase < 1 {
		deploymentK8sName = strings.TrimRight(deploymentK8sName[:min(len(deploymentK8sName), 40)], "-")
		maxBase = 63 - len(deploymentK8sName) - len(suffix) - 2
	}
	if len(base) > maxBase {
		base = strings.TrimRight(base[:maxBase], "-")
	}
	return deploymentK8sName + "-" + base + "-" + suffix, nil
}
