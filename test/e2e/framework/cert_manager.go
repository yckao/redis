package framework


import (
	"context"
	"fmt"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/log"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
)

const (
	IssuerName      = "e2e-self-signed-issuer"
	tlsCertFileName = "tls.crt"
	tlsKeyFileName  = "tls.key"
)

func (f *Framework) IssuerForDB(dbMeta, caSecretMeta metav1.ObjectMeta, resourceKind string) *cm_api.Issuer {
	thisIssuerName := rand.WithUniqSuffix(IssuerName)
	labelMap := map[string]string{
		api.LabelDatabaseName: dbMeta.Name,
		api.LabelDatabaseKind: resourceKind,
	}
	return &cm_api.Issuer{
		TypeMeta: metav1.TypeMeta{
			Kind: cm_api.IssuerKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      thisIssuerName,
			Namespace: dbMeta.Namespace,
			Labels:    labelMap,
		},
		Spec: cm_api.IssuerSpec{
			IssuerConfig: cm_api.IssuerConfig{
				CA: &cm_api.CAIssuer{
					SecretName: caSecretMeta.Name,
				},
			},
		},
	}
}

func (f *Framework) CreateIssuer(obj *cm_api.Issuer) (*cm_api.Issuer, error) {
	issuer, err := f.certManagerClient.CertmanagerV1alpha2().Issuers(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return issuer, err
}

func (f *Framework) UpdateIssuer(meta metav1.ObjectMeta, transformer func(cm_api.Issuer) cm_api.Issuer) error {
	attempt := 0
	for ; attempt < maxAttempts; attempt = attempt + 1 {
		cur, err := f.certManagerClient.CertmanagerV1alpha2().Issuers(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(err) {
			return nil
		} else if err == nil {
			modified := transformer(*cur)
			_, err = f.certManagerClient.CertmanagerV1alpha2().Issuers(cur.Namespace).Update(context.TODO(), &modified, metav1.UpdateOptions{})
			if err == nil {
				return nil
			}
		}
		log.Errorf("Attempt %d failed to update Issuer %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(updateRetryInterval)
	}
	return fmt.Errorf("failed to update Issuer %s@%s after %d attempts", meta.Name, meta.Namespace, attempt)
}

func (f *Framework) DeleteIssuer(meta metav1.ObjectMeta) error {
	return f.certManagerClient.CertmanagerV1alpha2().Issuers(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}

func (fi *Invocation) InsureIssuer(objectMeta metav1.ObjectMeta, kind string) (*cm_api.Issuer, error) {
	//create cert-manager ca secret
	clientCASecret := fi.SelfSignedCASecret(objectMeta, kind)
	secret, err := fi.CreateSecret(clientCASecret)
	if err != nil {
		return nil, err
	}
	fmt.Println(".................", secret.Name)
	fi.AppendToCleanupList(secret)
	//create issuer
	issuer := fi.IssuerForDB(objectMeta, clientCASecret.ObjectMeta, kind)
	issuer, err = fi.CreateIssuer(issuer)
	fi.AppendToCleanupList(issuer)
	return issuer, err
}
