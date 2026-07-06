package sqlcomment

import "os"

const environmentVar = "UNKEY_ENVIRONMENT"

// EnvironmentFromEnv returns the deployment environment name from UNKEY_ENVIRONMENT.
// The value is omitted from SQL comments when unset.
func EnvironmentFromEnv() string {
	return os.Getenv(environmentVar)
}
