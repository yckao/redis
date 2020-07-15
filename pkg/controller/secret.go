package controller

import (
	"context"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) secretExists(meta metav1.ObjectMeta) bool {
	_, err := c.Client.CoreV1().Secrets(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	return err == nil
}

func (c *Controller) RedisForSecret(s *core.Secret) cache.ExplicitKey {
	ctrl := metav1.GetControllerOf(s)
	if ctrl != nil {
		if ctrl.Kind == api.ResourceKindRedis {
			return cache.ExplicitKey(s.Namespace + "/" + ctrl.Name)
		}
		return ""
	}

	certName, ok := s.Annotations[cm_api.CertificateNameKey]
	if !ok {
		return ""
	}

	cert, err := c.CertManagerClient.CertmanagerV1alpha2().Certificates(s.Namespace).Get(context.TODO(), certName, metav1.GetOptions{})
	if err != nil {
		return ""
	}
	if cert.Spec.SecretName != s.Name {
		return ""
	}

	certCtrl := metav1.GetControllerOf(cert)
	if certCtrl == nil || certCtrl.Kind != api.ResourceKindRedis {
		return ""
	}
	return cache.ExplicitKey(s.Namespace + "/" + certCtrl.Name)
}
