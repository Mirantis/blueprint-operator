package webhook

// The following manifests contain a self-signed issuer CR and a certificate CR.
// More document can be found at https://docs.cert-manager.io

const certificateTemplate = `
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  labels:
    app.kubernetes.io/component: certificate
    app.kubernetes.io/created-by: boundless-operator
    app.kubernetes.io/instance: serving-cert
    app.kubernetes.io/name: certificate
    app.kubernetes.io/part-of: boundless-operator
  name: boundless-operator-serving-cert
  namespace: boundless-system
spec:
  dnsNames:
  - boundless-operator-webhook-service.boundless-system.svc
  - boundless-operator-webhook-service.boundless-system.svc.cluster.local
  issuerRef:
    kind: Issuer
    name: boundless-operator-selfsigned-issuer
  secretName: webhook-server-cert
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  labels:
    app.kubernetes.io/component: certificate
    app.kubernetes.io/created-by: boundless-operator
    app.kubernetes.io/instance: selfsigned-issuer
    app.kubernetes.io/name: issuer
    app.kubernetes.io/part-of: boundless-operator
  name: boundless-operator-selfsigned-issuer
  namespace: boundless-system
spec:
  selfSigned: {}
`
