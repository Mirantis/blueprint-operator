package funcs

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/mirantiscontainers/boundless-operator/api/v1alpha1"
)

// DefaultPollInterval is the suggested poll interval for wait.For.
const DefaultPollInterval = time.Millisecond * 500

// ResourceMatcher is a function that returns true if the supplied resource matches the desired state.
type ResourceMatcher func(object k8s.Object) bool

// AllOf runs the supplied functions in order
func AllOf(fns ...features.Func) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		for _, fn := range fns {
			ctx = fn(ctx, t, c)
		}
		return ctx
	}
}

// ApplyResources applies all manifests under the supplied directory that match
// the supplied glob pattern (e.g. *.yaml). It uses server-side apply - fields
// are managed by the supplied field manager. It fails the test if any supplied
// resource cannot be applied successfully.
func ApplyResources(manager, dir, pattern string, options ...decoder.DecodeOption) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dfs := os.DirFS(dir)

		files, _ := fs.Glob(dfs, pattern)
		if len(files) == 0 {
			t.Errorf("No resources found in %s", filepath.Join(dir, pattern))
			return ctx
		}

		if err := decoder.DecodeEachFile(ctx, dfs, pattern, ApplyHandler(c.Client().Resources(), manager), options...); err != nil {
			t.Fatal(err)
			return ctx
		}

		t.Logf("Applied resources from %s (matched %d manifests)", filepath.Join(dir, pattern), len(files))
		return ctx
	}
}

// DeleteResources deletes (from the environment) all resources defined by the
// manifests under the supplied directory that match the supplied glob pattern
// (e.g. *.yaml).
func DeleteResources(dir, pattern string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dfs := os.DirFS(dir)

		if err := decoder.DecodeEachFile(ctx, dfs, pattern, decoder.DeleteHandler(c.Client().Resources())); err != nil {
			t.Fatal(err)
			return ctx
		}

		files, _ := fs.Glob(dfs, pattern)
		t.Logf("Deleted resources from %s (matched %d manifests)", filepath.Join(dir, pattern), len(files))
		return ctx
	}
}

// DeleteResource deletes (from the environment) the supplied resource.
func DeleteResource(o k8s.Object) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		if err := c.Client().Resources().Delete(ctx, o); err != nil {
			t.Errorf("failed to delete resource %s: %v", identifier(o), err)
			return ctx
		}

		t.Logf("Deleted resource %s", identifier(o))
		return ctx
	}
}

// ResourcesCreatedWithin fails a test if the supplied resources are not found
// to exist within the supplied duration.
func ResourcesCreatedWithin(d time.Duration, dir, pattern string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {

		rs, err := decoder.DecodeAllFiles(ctx, os.DirFS(dir), pattern)
		if err != nil {
			t.Error(err)
			return ctx
		}

		list := &unstructured.UnstructuredList{}
		for _, o := range rs {
			u := asUnstructured(o)
			list.Items = append(list.Items, *u)
			t.Logf("Waiting %s for %s to exist...", d, identifier(u))
		}

		start := time.Now()
		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesFound(list), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Errorf("resources did not exist: %v", err)
			return ctx
		}

		t.Logf("%d resources found to exist after %s", len(rs), since(start))
		return ctx
	}
}

// ResourcesDeletedWithin fails a test if the supplied resources are not deleted
// within the supplied duration.
func ResourcesDeletedWithin(d time.Duration, dir, pattern string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {

		rs, err := decoder.DecodeAllFiles(ctx, os.DirFS(dir), pattern)
		if err != nil {
			t.Error(err)
			return ctx
		}

		list := &unstructured.UnstructuredList{}
		for _, o := range rs {
			u := asUnstructured(o)
			list.Items = append(list.Items, *u)
			t.Logf("Waiting %s for %s to be deleted...", d, identifier(u))
		}

		start := time.Now()
		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesDeleted(list), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Errorf("resources not deleted after waiting for %s: %v", d.String(), err)
			return ctx
		}

		t.Logf("%d resources deleted after %s", len(rs), since(start))
		return ctx
	}
}

// ResourceMatchWithin fails a test if the supplied resource does not match the
// supplied matcher within the supplied duration.
func ResourceMatchWithin(d time.Duration, o k8s.Object, matcher ResourceMatcher) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		start := time.Now()
		if err := wait.For(conditions.New(c.Client().Resources()).ResourceMatch(o, matcher), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Errorf("resource %s did not match the condition before timeout (%s): %v", identifier(o), d.String(), err)
			return ctx
		}

		t.Logf("resource %s matched after %s", identifier(o), since(start))
		return ctx
	}
}

// ResourceDeletedWithin fails a test if the supplied resource is not deleted
// within the supplied duration.
func ResourceDeletedWithin(d time.Duration, o k8s.Object) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		t.Logf("Waiting %s for %s to be deleted...", d, identifier(o))

		start := time.Now()
		if err := wait.For(conditions.New(c.Client().Resources()).ResourceDeleted(o), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Errorf("resource %s not deleted: %v", identifier(o), err)
			return ctx
		}

		t.Logf("resource %s deleted after %s", identifier(o), since(start))
		return ctx
	}
}

// AddonHaveStatusWithin fails a test if the supplied addon do not
// have (i.e. become) the supplied status within the supplied duration.
func AddonHaveStatusWithin(d time.Duration, addon *v1alpha1.Addon, desired v1alpha1.StatusType) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		statusMatcherFunc := func(object k8s.Object) bool {
			a := object.(*v1alpha1.Addon)
			return a.Status.Type == desired
		}

		start := time.Now()
		if err := wait.For(conditions.New(c.Client().Resources()).ResourceMatch(addon, statusMatcherFunc), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Fatalf("addon %s did not have desired status type '%s' in %s: %v", identifier(addon), desired, since(start), err)
			return ctx
		}

		t.Logf("addon %s have desired status type '%s' after %s", identifier(addon), desired, since(start))
		return ctx
	}
}

// DeploymentBecomesAvailableWithin fails a test if the supplied Deployment is
// not Available within the supplied duration.
func DeploymentBecomesAvailableWithin(d time.Duration, namespace, name string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		dp := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		t.Logf("Waiting %s for deployment %s/%s to become Available...", d, dp.GetNamespace(), dp.GetName())
		start := time.Now()
		if err := wait.For(conditions.New(c.Client().Resources()).DeploymentConditionMatch(dp, appsv1.DeploymentAvailable, corev1.ConditionTrue), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Fatal(err)
			return ctx
		}
		t.Logf("Deployment %s/%s is Available after %s", dp.GetNamespace(), dp.GetName(), since(start))
		return ctx
	}
}

func AddonResourcesCreatedWithin(d time.Duration, addons ...*v1alpha1.Addon) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		list := &unstructured.UnstructuredList{}
		for _, o := range addons {
			u := asUnstructured(o)
			list.Items = append(list.Items, *u)
			t.Logf("Waiting %s for %s to exist...", d, identifier(u))
		}

		start := time.Now()
		if err := wait.For(conditions.New(c.Client().Resources()).ResourcesFound(list), wait.WithTimeout(d), wait.WithInterval(DefaultPollInterval)); err != nil {
			t.Errorf("resources did not exist: %v", err)
			return ctx
		}

		t.Logf("%d resources found to exist after %s", len(list.Items), since(start))
		return ctx
	}
}

// ApplyHandler is a decoder.Handler that uses server-side apply to apply the
// supplied object.
func ApplyHandler(r *resources.Resources, manager string) decoder.HandlerFunc {
	return func(ctx context.Context, obj k8s.Object) error {
		if err := r.GetControllerRuntimeClient().Patch(ctx, obj, client.Apply, client.FieldOwner(manager), client.ForceOwnership); err != nil {
			return err
		}
		return nil
	}
}

// asUnstructured turns an arbitrary runtime.Object into an *Unstructured. If
// it's already a concrete *Unstructured it just returns it, otherwise it
// round-trips it through JSON encoding. This is necessary because types that
// are registered with our scheme will be returned as Objects backed by the
// concrete type, whereas types that are not will be returned as *Unstructured.
func asUnstructured(o runtime.Object) *unstructured.Unstructured {
	if u, ok := o.(*unstructured.Unstructured); ok {
		return u
	}

	u := &unstructured.Unstructured{}
	j, _ := json.Marshal(o)
	_ = json.Unmarshal(j, u)
	return u
}

// identifier returns the supplied resource's kind, group, name, and (if any)
// namespace.
func identifier(o k8s.Object) string {
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

func since(t time.Time) string {
	return fmt.Sprintf("%.3fs", time.Since(t).Seconds())
}
