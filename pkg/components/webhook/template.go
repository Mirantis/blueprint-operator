package webhook

const webhookTemplate = `
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app.kubernetes.io/name: serviceaccount
    app.kubernetes.io/instance: controller-manager
    app.kubernetes.io/component: rbac
    app.kubernetes.io/created-by: blueprint-operator
    app.kubernetes.io/part-of: blueprint-operator
  name: blueprint-webhook-service-account
  namespace: blueprint-system

---
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: blueprint-operator
    app.kubernetes.io/instance: webhook-service
    app.kubernetes.io/name: service
    app.kubernetes.io/part-of: blueprint-operator
  name: blueprint-operator-webhook-service
  namespace: blueprint-system
spec:
  ports:
    - port: 443
      protocol: TCP
      targetPort: 9443
  selector:
    app.kubernetes.io/name: blueprint-operator-webhook
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  annotations:
    cert-manager.io/inject-ca-from: blueprint-system/blueprint-operator-serving-cert
  labels:
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: blueprint-operator
    app.kubernetes.io/instance: mutating-webhook-configuration
    app.kubernetes.io/name: mutatingwebhookconfiguration
    app.kubernetes.io/part-of: blueprint-operator
  name: blueprint-operator-mutating-webhook-configuration
webhooks:
  - admissionReviewVersions:
      - v1
    clientConfig:
      service:
        name: blueprint-operator-webhook-service
        namespace: blueprint-system
        path: /mutate-blueprint-mirantis-com-v1alpha1-blueprint
    failurePolicy: Fail
    name: mblueprint.kb.io
    rules:
      - apiGroups:
          - blueprint.mirantis.com
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
    cert-manager.io/inject-ca-from: blueprint-system/blueprint-operator-serving-cert
  labels:
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: blueprint-operator
    app.kubernetes.io/instance: validating-webhook-configuration
    app.kubernetes.io/name: validatingwebhookconfiguration
    app.kubernetes.io/part-of: blueprint-operator
  name: blueprint-operator-validating-webhook-configuration
webhooks:
  - admissionReviewVersions:
      - v1
    clientConfig:
      service:
        name: blueprint-operator-webhook-service
        namespace: blueprint-system
        path: /validate-blueprint-mirantis-com-v1alpha1-blueprint
    failurePolicy: Fail
    name: vblueprint.kb.io
    rules:
      - apiGroups:
          - blueprint.mirantis.com
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
  name: blueprint-operator-webhook
  namespace: blueprint-system
  labels:
    app.kubernetes.io/name: deployment
    app.kubernetes.io/instance: blueprint-operator
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: blueprint-operator
    app.kubernetes.io/part-of: blueprint-operator
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: blueprint-operator-webhook
  replicas: 1
  template:
    metadata:
        labels:
            app.kubernetes.io/name: blueprint-operator-webhook
    spec:
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      tolerations:
      - key: "node-role.kubernetes.io/master"
        operator: "Exists"
      - effect: "NoSchedule"
        operator: "Exists"
      - effect: "NoExecute"
        operator: "Exists"
      containers:
        - command:
            - /manager
          args:
            - --webhook
          image: {{.Image}}
          name: blueprint-operator-webhook
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
            secretName: blueprint-webhook-server-cert
      serviceAccountName: blueprint-webhook-service-account
      terminationGracePeriodSeconds: 10

`
