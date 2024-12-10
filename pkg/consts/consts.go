package consts

const (
	// NamespaceBlueprintSystem is the namespace where all blueprint components are installed
	NamespaceBlueprintSystem = "blueprint-system"

	// NamespaceFluxSystem is the namespace where all flux components are installed
	NamespaceFluxSystem = "flux-system"

	// BlueprintOperatorName is the name of the blueprint operator deployment
	BlueprintOperatorName = "blueprint-operator-controller-manager"

	// BlueprintOperatorWebhookName is the name of the blueprint operator webhook deployment
	BlueprintOperatorWebhookName = "blueprint-operator-webhook"

	// BlueprintContainerName is the name of the blueprint operator container
	BlueprintContainerName = "manager"

	// ManagedByLabel is the label used to identify resources managed by the blueprint operator
	ManagedByLabel = "app.kubernetes.io/managed-by"

	// ManagedByValue is the label value used to identify resources managed by the blueprint operator
	ManagedByValue = "blueprint-operator"
)
