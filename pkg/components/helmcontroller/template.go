package helmcontroller

// Source: https://github.com/cert-manager/cert-manager/releases/download/v1.9.1/cert-manager.yaml

const helmControllerTemplate = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: helm-controller
  namespace: boundless-system
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: helmcharts.helm.cattle.io
spec:
  group: helm.cattle.io
  names:
    kind: HelmChart
    plural: helmcharts
    singular: helmchart
  preserveUnknownFields: false
  scope: Namespaced
  versions:
    - additionalPrinterColumns:
        - jsonPath: .status.jobName
          name: Job
          type: string
        - jsonPath: .spec.chart
          name: Chart
          type: string
        - jsonPath: .spec.targetNamespace
          name: TargetNamespace
          type: string
        - jsonPath: .spec.version
          name: Version
          type: string
        - jsonPath: .spec.repo
          name: Repo
          type: string
        - jsonPath: .spec.helmVersion
          name: HelmVersion
          type: string
        - jsonPath: .spec.bootstrap
          name: Bootstrap
          type: string
        - jsonPath: .spec.dryRun
          name: DryRun
          type: string
      name: v1
      schema:
        openAPIV3Schema:
          properties:
            spec:
              properties:
                dryRun:
                  nullable: true
                  type: string
                bootstrap:
                  type: boolean
                chart:
                  nullable: true
                  type: string
                chartContent:
                  nullable: true
                  type: string
                failurePolicy:
                  nullable: true
                  type: string
                helmVersion:
                  nullable: true
                  type: string
                jobImage:
                  nullable: true
                  type: string
                repo:
                  nullable: true
                  type: string
                repoCA:
                  nullable: true
                  type: string
                set:
                  additionalProperties:
                    nullable: true
                    type: string
                  nullable: true
                  type: object
                targetNamespace:
                  nullable: true
                  type: string
                timeout:
                  nullable: true
                  type: string
                valuesContent:
                  nullable: true
                  type: string
                version:
                  nullable: true
                  type: string
              type: object
            status:
              properties:
                jobName:
                  nullable: true
                  type: string
              type: object
          type: object
      served: true
      storage: true

---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: helmchartconfigs.helm.cattle.io
spec:
  group: helm.cattle.io
  names:
    kind: HelmChartConfig
    plural: helmchartconfigs
    singular: helmchartconfig
  preserveUnknownFields: false
  scope: Namespaced
  versions:
    - name: v1
      schema:
        openAPIV3Schema:
          properties:
            spec:
              properties:
                failurePolicy:
                  nullable: true
                  type: string
                valuesContent:
                  nullable: true
                  type: string
              type: object
          type: object
      served: true
      storage: true
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: helmreleases.helm.cattle.io
spec:
  group: helm.cattle.io
  names:
    kind: HelmRelease
    plural: helmreleases
    singular: helmrelease
  preserveUnknownFields: false
  scope: Namespaced
  versions:
    - additionalPrinterColumns:
        - jsonPath: .spec.release.name
          name: Release Name
          type: string
        - jsonPath: .spec.release.namespace
          name: Release Namespace
          type: string
        - jsonPath: .status.version
          name: Version
          type: string
        - jsonPath: .status.state
          name: State
          type: string
      name: v1alpha1
      schema:
        openAPIV3Schema:
          properties:
            spec:
              properties:
                release:
                  properties:
                    name:
                      nullable: true
                      type: string
                    namespace:
                      nullable: true
                      type: string
                  type: object
              type: object
            status:
              properties:
                conditions:
                  items:
                    properties:
                      lastTransitionTime:
                        nullable: true
                        type: string
                      lastUpdateTime:
                        nullable: true
                        type: string
                      message:
                        nullable: true
                        type: string
                      reason:
                        nullable: true
                        type: string
                      status:
                        nullable: true
                        type: string
                      type:
                        nullable: true
                        type: string
                    type: object
                  nullable: true
                  type: array
                description:
                  nullable: true
                  type: string
                notes:
                  nullable: true
                  type: string
                state:
                  nullable: true
                  type: string
                version:
                  type: integer
              type: object
          type: object
      served: true
      storage: true
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: helm-controller
rules:
  - apiGroups:
      - helmcharts.helm.cattle.io
    resources:
      - '*'
    verbs:
      - '*'
  - apiGroups:
      - helmchartconfigs.helm.cattle.io
    resources:
      - '*'
    verbs:
      - '*'
  - apiGroups:
      - helmreleases.helm.cattle.io
    resources:
      - '*'
    verbs:
      - '*'
  - apiGroups:
      - ""
    resources:
      - namespaces
      - secrets
      - configmaps
      - serviceaccounts
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - events
    verbs:
      - create
      - patch
  - apiGroups:
      - ""
    resources:
      - configmaps
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
  - apiGroups:
      - ""
    resources:
      - configmaps/status
    verbs:
      - get
      - update
      - patch
  - apiGroups:
      - coordination.k8s.io
    resources:
      - leases
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: helm-controller-cluster-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: helm-controller
    namespace: boundless-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: helm-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: helm-controller
subjects:
  - kind: ServiceAccount
    name: helm-controller
    namespace: boundless-system
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: helm-controller
  namespace: boundless-system
  labels:
    app: helm-controller
spec:
  replicas: 1
  selector:
    matchLabels:
      app: helm-controller
  template:
    metadata:
      labels:
        app: helm-controller
    spec:
      containers:
        - name: helm-controller
          image: tppolkow/helm-controller:test3
          command: ["helm-controller"]
      serviceAccountName: helm-controller
`
