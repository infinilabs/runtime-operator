// pkg/apis/common/constants.go
package common

// Constants for standard labels and operator identification.
const (
	AppNameLabel      = "app.infini.cloud/application-name"   // Label for ApplicationDefinition name
	CompNameLabel     = "app.infini.cloud/component-name"     // Label for ComponentDefinition name (type)
	CompInstanceLabel = "app.infini.cloud/component-instance" // Label for ApplicationComponent name (instance)
	ManagedByLabel    = "app.kubernetes.io/managed-by"        // Standard Kubernetes label
	OperatorName      = "infini-operator"                     // Name of this operator
)
