package manifests

const CRDPatchTemplate = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    cert-manager.io/inject-ca-from: boundless-system/boundless-operator-serving-cert
    controller-gen.kubebuilder.io/version: v0.11.1
  name: addons.boundless.mirantis.com
spec:
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        service:
          name: boundless-operator-webhook-service
          namespace: boundless-system
          path: /convert
      conversionReviewVersions:
      - v1
  group: boundless.mirantis.com
  names:
    kind: Addon
    listKind: AddonList
    plural: addons
    singular: addon
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - description: Whether the component is running and stable.
      jsonPath: .status.type
      name: Status
      type: string
    name: v1alpha1
    schema:
      openAPIV3Schema:
        description: Addon is the Schema for the addons API
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: AddonSpec defines the desired state of Addon
            properties:
              chart:
                properties:
                  name:
                    type: string
                  repo:
                    type: string
                  set:
                    additionalProperties:
                      anyOf:
                      - type: integer
                      - type: string
                      x-kubernetes-int-or-string: true
                    type: object
                  values:
                    type: string
                  version:
                    type: string
                required:
                - name
                - repo
                - version
                type: object
              enabled:
                type: boolean
              kind:
                enum:
                - manifest
                - chart
                - Manifest
                - Chart
                type: string
              manifest:
                properties:
                  url:
                    minLength: 1
                    type: string
                required:
                - url
                type: object
              name:
                type: string
              namespace:
                type: string
            required:
            - enabled
            - kind
            - name
            type: object
          status:
            description: AddonStatus defines the observed state of Addon
            properties:
              lastTransitionTime:
                description: The timestamp representing the start time for the current
                  status.
                format: date-time
                type: string
              message:
                description: Optionally, a detailed message providing additional context.
                type: string
              reason:
                description: A brief reason explaining the condition.
                type: string
              type:
                description: The type of condition. May be Available, Progressing,
                  or Degraded.
                type: string
            required:
            - lastTransitionTime
            - type
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    cert-manager.io/inject-ca-from: boundless-system/boundless-operator-serving-cert
    controller-gen.kubebuilder.io/version: v0.11.1
  name: blueprints.boundless.mirantis.com
spec:
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        service:
          name: boundless-operator-webhook-service
          namespace: boundless-system
          path: /convert
      conversionReviewVersions:
      - v1
  group: boundless.mirantis.com
  names:
    kind: Blueprint
    listKind: BlueprintList
    plural: blueprints
    singular: blueprint
  scope: Namespaced
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        description: Blueprint is the Schema for the blueprints API
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: BlueprintSpec defines the desired state of Blueprint
            properties:
              components:
                description: Components contains all the components that should be
                  installed
                properties:
                  addons:
                    items:
                      description: AddonSpec defines the desired state of Addon
                      properties:
                        chart:
                          properties:
                            name:
                              type: string
                            repo:
                              type: string
                            set:
                              additionalProperties:
                                anyOf:
                                - type: integer
                                - type: string
                                x-kubernetes-int-or-string: true
                              type: object
                            values:
                              type: string
                            version:
                              type: string
                          required:
                          - name
                          - repo
                          - version
                          type: object
                        enabled:
                          type: boolean
                        kind:
                          enum:
                          - manifest
                          - chart
                          - Manifest
                          - Chart
                          type: string
                        manifest:
                          properties:
                            url:
                              minLength: 1
                              type: string
                          required:
                          - url
                          type: object
                        name:
                          type: string
                        namespace:
                          type: string
                      required:
                      - enabled
                      - kind
                      - name
                      type: object
                    type: array
                  core:
                    properties:
                      ingress:
                        description: IngressSpec defines the desired state of Ingress
                        properties:
                          config:
                            type: string
                          enabled:
                            description: Enabled is a flag to enable/disable the ingress
                            type: boolean
                          provider:
                            type: string
                        required:
                        - enabled
                        - provider
                        type: object
                    type: object
                type: object
            type: object
          status:
            description: BlueprintStatus defines the observed state of Blueprint
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    cert-manager.io/inject-ca-from: boundless-system/boundless-operator-serving-cert
    controller-gen.kubebuilder.io/version: v0.11.1
  name: ingresses.boundless.mirantis.com
spec:
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        service:
          name: boundless-operator-webhook-service
          namespace: boundless-system
          path: /convert
      conversionReviewVersions:
      - v1
  group: boundless.mirantis.com
  names:
    kind: Ingress
    listKind: IngressList
    plural: ingresses
    singular: ingress
  scope: Namespaced
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        description: Ingress is the Schema for the ingresses API
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: IngressSpec defines the desired state of Ingress
            properties:
              config:
                type: string
              enabled:
                description: Enabled is a flag to enable/disable the ingress
                type: boolean
              provider:
                type: string
            required:
            - enabled
            - provider
            type: object
          status:
            description: IngressStatus defines the observed state of Ingress
            properties:
              ingressReady:
                type: boolean
            required:
            - ingressReady
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    cert-manager.io/inject-ca-from: boundless-system/boundless-operator-serving-cert
    controller-gen.kubebuilder.io/version: v0.11.1
  name: manifests.boundless.mirantis.com
spec:
  conversion:
    strategy: Webhook
    webhook:
      clientConfig:
        service:
          name: boundless-operator-webhook-service
          namespace: boundless-system
          path: /convert
      conversionReviewVersions:
      - v1
  group: boundless.mirantis.com
  names:
    kind: Manifest
    listKind: ManifestList
    plural: manifests
    singular: manifest
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - description: Whether the component is running and stable.
      jsonPath: .status.type
      name: Status
      type: string
    name: v1alpha1
    schema:
      openAPIV3Schema:
        description: Manifest is the Schema for the manifests API
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: ManifestSpec defines the desired state of Manifest
            properties:
              checksum:
                type: string
              newChecksum:
                type: string
              objects:
                items:
                  description: ManifestObject consists of the fields required to update/delete
                    an object
                  properties:
                    group:
                      type: string
                    kind:
                      type: string
                    name:
                      type: string
                    namespace:
                      type: string
                    version:
                      type: string
                  required:
                  - group
                  - kind
                  - name
                  - namespace
                  - version
                  type: object
                type: array
              url:
                description: 'INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
                  Important: Run "make" to regenerate code after modifying this file'
                type: string
            required:
            - checksum
            - url
            type: object
          status:
            description: ManifestStatus defines the observed state of Manifest
            properties:
              lastTransitionTime:
                description: The timestamp representing the start time for the current
                  status.
                format: date-time
                type: string
              message:
                description: Optionally, a detailed message providing additional context.
                type: string
              reason:
                description: A brief reason explaining the condition.
                type: string
              type:
                description: The type of condition. May be Available, Progressing,
                  or Degraded.
                type: string
            required:
            - lastTransitionTime
            - type
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
`
