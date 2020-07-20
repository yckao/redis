/*
Copyright AppsCode Inc. and Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"context"
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	cs "kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kutil "kmodules.xyz/client-go"
)

func CreateOrPatchProxySQL(ctx context.Context, c cs.KubedbV1alpha1Interface, meta metav1.ObjectMeta, transform func(*api.ProxySQL) *api.ProxySQL, opts metav1.PatchOptions) (*api.ProxySQL, kutil.VerbType, error) {
	cur, err := c.ProxySQLs(meta.Namespace).Get(ctx, meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating ProxySQL %s/%s.", meta.Namespace, meta.Name)
		out, err := c.ProxySQLs(meta.Namespace).Create(ctx, transform(&api.ProxySQL{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ProxySQL",
				APIVersion: api.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}), metav1.CreateOptions{
			DryRun:       opts.DryRun,
			FieldManager: opts.FieldManager,
		})
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchProxySQL(ctx, c, cur, transform, opts)
}

func PatchProxySQL(ctx context.Context, c cs.KubedbV1alpha1Interface, cur *api.ProxySQL, transform func(*api.ProxySQL) *api.ProxySQL, opts metav1.PatchOptions) (*api.ProxySQL, kutil.VerbType, error) {
	return PatchProxySQLObject(ctx, c, cur, transform(cur.DeepCopy()), opts)
}

func PatchProxySQLObject(ctx context.Context, c cs.KubedbV1alpha1Interface, cur, mod *api.ProxySQL, opts metav1.PatchOptions) (*api.ProxySQL, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := jsonpatch.CreateMergePatch(curJson, modJson)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching ProxySQL %s/%s with %s.", cur.Namespace, cur.Name, string(patch))
	out, err := c.ProxySQLs(cur.Namespace).Patch(ctx, cur.Name, types.MergePatchType, patch, opts)
	return out, kutil.VerbPatched, err
}

func TryUpdateProxySQL(ctx context.Context, c cs.KubedbV1alpha1Interface, meta metav1.ObjectMeta, transform func(*api.ProxySQL) *api.ProxySQL, opts metav1.UpdateOptions) (result *api.ProxySQL, err error) {
	attempt := 0
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.ProxySQLs(meta.Namespace).Get(ctx, meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(e2) {
			return false, e2
		} else if e2 == nil {

			result, e2 = c.ProxySQLs(cur.Namespace).Update(ctx, transform(cur.DeepCopy()), opts)
			return e2 == nil, nil
		}
		glog.Errorf("Attempt %d failed to update ProxySQL %s/%s due to %v.", attempt, cur.Namespace, cur.Name, e2)
		return false, nil
	})

	if err != nil {
		err = fmt.Errorf("failed to update ProxySQL %s/%s after %d attempts due to %v", meta.Namespace, meta.Name, attempt, err)
	}
	return
}

func UpdateProxySQLStatus(
	ctx context.Context,
	c cs.KubedbV1alpha1Interface,
	meta metav1.ObjectMeta,
	transform func(*api.ProxySQLStatus) *api.ProxySQLStatus,
	opts metav1.UpdateOptions,
) (result *api.ProxySQL, err error) {
	apply := func(x *api.ProxySQL) *api.ProxySQL {
		return &api.ProxySQL{
			TypeMeta:   x.TypeMeta,
			ObjectMeta: x.ObjectMeta,
			Spec:       x.Spec,
			Status:     *transform(x.Status.DeepCopy()),
		}
	}

	attempt := 0
	cur, err := c.ProxySQLs(meta.Namespace).Get(ctx, meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		var e2 error
		result, e2 = c.ProxySQLs(meta.Namespace).UpdateStatus(ctx, apply(cur), opts)
		if kerr.IsConflict(e2) {
			latest, e3 := c.ProxySQLs(meta.Namespace).Get(ctx, meta.Name, metav1.GetOptions{})
			switch {
			case e3 == nil:
				cur = latest
				return false, nil
			case kutil.IsRequestRetryable(e3):
				return false, nil
			default:
				return false, e3
			}
		} else if err != nil && !kutil.IsRequestRetryable(e2) {
			return false, e2
		}
		return e2 == nil, nil
	})

	if err != nil {
		err = fmt.Errorf("failed to update status of ProxySQL %s/%s after %d attempts due to %v", meta.Namespace, meta.Name, attempt, err)
	}
	return
}
