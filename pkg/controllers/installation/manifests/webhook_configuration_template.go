package manifests

const WebhookConfigTemplate = `
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: boundless-operator
    app.kubernetes.io/instance: webhook-service
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: service
    app.kubernetes.io/part-of: boundless-operator
  name: boundless-operator-webhook-service
  namespace: boundless-system
spec:
  ports:
  - port: 443
    protocol: TCP
    targetPort: 9443
  selector:
    control-plane: controller-manager
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  annotations:
    cert-manager.io/inject-ca-from: boundless-system/boundless-operator-serving-cert
  creationTimestamp: null
  labels:
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: boundless-operator
    app.kubernetes.io/instance: mutating-webhook-configuration
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: mutatingwebhookconfiguration
    app.kubernetes.io/part-of: boundless-operator
  name: boundless-operator-mutating-webhook-configuration
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    service:
      name: boundless-operator-webhook-service
      namespace: boundless-system
      path: /mutate-boundless-mirantis-com-v1alpha1-blueprint
  failurePolicy: Fail
  name: mblueprint.kb.io
  rules:
  - apiGroups:
    - boundless.mirantis.com
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - blueprints
  sideEffects: None
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  annotations:
    cert-manager.io/inject-ca-from: boundless-system/boundless-operator-serving-cert
  creationTimestamp: null
  labels:
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: boundless-operator
    app.kubernetes.io/instance: validating-webhook-configuration
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: validatingwebhookconfiguration
    app.kubernetes.io/part-of: boundless-operator
  name: boundless-operator-validating-webhook-configuration
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    service:
      name: boundless-operator-webhook-service
      namespace: boundless-system
      path: /validate-boundless-mirantis-com-v1alpha1-blueprint
  failurePolicy: Fail
  name: vblueprint.kb.io
  rules:
  - apiGroups:
    - boundless.mirantis.com
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - blueprints
  sideEffects: None
`
