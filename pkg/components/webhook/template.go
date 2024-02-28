package webhook

const webhookTemplate = `
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app.kubernetes.io/name: serviceaccount
    app.kubernetes.io/instance: controller-manager
    app.kubernetes.io/component: rbac
    app.kubernetes.io/created-by: boundless-operator
    app.kubernetes.io/part-of: boundless-operator
  name: boundless-webhook-service-account
  namespace: boundless-system

---
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: boundless-operator
    app.kubernetes.io/instance: webhook-service
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
    app.kubernetes.io/name: boundless-operator-webhook
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  annotations:
    cert-manager.io/inject-ca-from: boundless-system/boundless-operator-serving-cert
  labels:
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: boundless-operator
    app.kubernetes.io/instance: mutating-webhook-configuration
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
  labels:
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: boundless-operator
    app.kubernetes.io/instance: validating-webhook-configuration
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
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: boundless-operator-webhook
  namespace: boundless-system
  labels:
    app.kubernetes.io/name: deployment
    app.kubernetes.io/instance: boundless-operator
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: boundless-operator
    app.kubernetes.io/part-of: boundless-operator
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: boundless-operator-webhook
  replicas: 1
  template:
    metadata:
        labels:
            app.kubernetes.io/name: boundless-operator-webhook
    spec:
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      containers:
        - command:
            - /manager
          args:
            - --webhook
          image: {{.Image}}
          name: boundless-operator-webhook
          ports:
            - containerPort: 9443
              name: webhook-server
              protocol: TCP
          volumeMounts:
            - mountPath: /tmp/k8s-webhook-server/serving-certs
              name: webhook-certs
              readOnly: true
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - "ALL"
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8081
            initialDelaySeconds: 15
            periodSeconds: 20
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8081
            initialDelaySeconds: 5
            periodSeconds: 10
      volumes:
        - name: webhook-certs
          secret:
            defaultMode: 420
            secretName: boundless-webhook-server-cert
      serviceAccountName: boundless-webhook-service-account
      terminationGracePeriodSeconds: 10

`
