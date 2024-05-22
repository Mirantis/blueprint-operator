package webhook

// The following manifests contain a self-signed issuer CR and a certificate CR.
// More document can be found at https://docs.cert-manager.io

const certificateTemplate = `
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  labels:
    app.kubernetes.io/component: certificate
    app.kubernetes.io/created-by: blueprint-operator
    app.kubernetes.io/instance: serving-cert
    app.kubernetes.io/name: certificate
    app.kubernetes.io/part-of: blueprint-operator
  name: blueprint-operator-serving-cert
  namespace: blueprint-system
spec:
  dnsNames:
  - blueprint-operator-webhook-service.blueprint-system.svc
  - blueprint-operator-webhook-service.blueprint-system.svc.cluster.local
  issuerRef:
    kind: Issuer
    name: blueprint-operator-selfsigned-issuer
  secretName: blueprint-webhook-server-cert
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  labels:
    app.kubernetes.io/component: certificate
    app.kubernetes.io/created-by: blueprint-operator
    app.kubernetes.io/instance: selfsigned-issuer
    app.kubernetes.io/name: issuer
    app.kubernetes.io/part-of: blueprint-operator
  name: blueprint-operator-selfsigned-issuer
  namespace: blueprint-system
spec:
  selfSigned: {}
`
