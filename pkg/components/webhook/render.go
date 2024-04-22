package webhook

//
//const (
//	WebhookServiceAccountName = "boundless-webhook-service-account"
//)
//
//var commonLabels = map[string]string{
//	"app.kubernetes.io/name": "serviceaccount",
//}
//
//type webhookComponent struct {
//	webhookImage string
//}
//
//func (w *webhookComponent) Objects() []runtime.Object {
//	return []runtime.Object{
//		w.serviceAccount(),
//	}
//}
//
//func (w *webhookComponent) serviceAccount() *corev1.ServiceAccount {
//	return &corev1.ServiceAccount{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      WebhookServiceAccountName,
//			Namespace: consts.NamespaceBoundlessSystem,
//			Labels: map[string]string{
//				"app.kubernetes.io/name":       "serviceaccount",
//				"app.kubernetes.io/instance":   "controller-manager",
//				"app.kubernetes.io/component":  "rbac",
//				"app.kubernetes.io/created-by": "boundless-operator",
//				"app.kubernetes.io/part-of":    "boundless-operator",
//			},
//		},
//	}
//}
//
///*
//apiVersion: v1
//kind: Service
//metadata:
//  labels:
//    app.kubernetes.io/component: webhook
//    app.kubernetes.io/created-by: boundless-operator
//    app.kubernetes.io/instance: webhook-service
//    app.kubernetes.io/name: service
//    app.kubernetes.io/part-of: boundless-operator
//  name: boundless-operator-webhook-service
//  namespace: boundless-system
//spec:
//  ports:
//    - port: 443
//      protocol: TCP
//      targetPort: 9443
//  selector:
//    app.kubernetes.io/name: boundless-operator-webhook
//*/
//
//func (w *webhookComponent) service() *corev1.Service {
//	return &corev1.Service{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      "boundless-operator-webhook-service",
//			Namespace: consts.NamespaceBoundlessSystem,
//			Labels: map[string]string{
//				"app.kubernetes.io/name":       "service",
//				"app.kubernetes.io/instance":   "webhook-service",
//				"app.kubernetes.io/component":  "webhook",
//				"app.kubernetes.io/created-by": "boundless-operator",
//				"app.kubernetes.io/part-of":    "boundless-operator",
//			},
//		},
//		Spec: corev1.ServiceSpec{
//			Ports: []corev1.ServicePort{
//				{
//					Port:       443,
//					Protocol:   corev1.ProtocolTCP,
//					TargetPort: 9443,
//				},
//			},
//			Selector: map[string]string{
//				"app.kubernetes.io/name": "boundless-operator-webhook",
//			},
//		},
//	}
//}
