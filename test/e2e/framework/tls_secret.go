package framework

import (
"context"
api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

core "k8s.io/api/core/v1"
metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (f *Framework) CreateSecret(obj *core.Secret) (*core.Secret, error) {
	return f.kubeClient.CoreV1().Secrets(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}

func (f *Framework) SelfSignedCASecret(meta metav1.ObjectMeta, kind string) *core.Secret {
	labelMap := map[string]string{
		api.LabelDatabaseName: meta.Name,
		api.LabelDatabaseKind: kind,
	}
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name + "-self-signed-ca",
			Namespace: meta.Namespace,
			Labels:    labelMap,
		},
		Data: map[string][]byte{
			"tls.crt": f.CertStore.CACertBytes(),
			"tls.key": f.CertStore.CAKeyBytes(),
		},
	}
}

