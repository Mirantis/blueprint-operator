package utils

import (
	"fmt"
	"reflect"

	"sigs.k8s.io/e2e-framework/klient/k8s"
)

// GetIdentifier returns the supplied resource's kind, group, name, and (if any)
// namespace.
func GetIdentifier(o k8s.Object) string {
	k := o.GetObjectKind().GroupVersionKind().Kind
	if k == "" {
		t := reflect.TypeOf(o)
		if t != nil {
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}
			k = t.Name()
		} else {
			k = fmt.Sprintf("%T", o)
		}
	}
	groupSuffix := ""
	if g := o.GetObjectKind().GroupVersionKind().Group; g != "" {
		groupSuffix = "." + g
	}
	if o.GetNamespace() == "" {
		return fmt.Sprintf("%s%s %s", k, groupSuffix, o.GetName())
	}
	return fmt.Sprintf("%s%s %s/%s", k, groupSuffix, o.GetNamespace(), o.GetName())
}
