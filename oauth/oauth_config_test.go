//
// Copyright (c) 2021 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oauth

import (
	"context"
	"fmt"
	"testing"

	"github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"

	"github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/config"
	oauthstate2 "github.com/redhat-appstudio/service-provider-integration-operator/pkg/spi-shared/oauthstate"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testClientId     = "test_client_id_123"
	testClientSecret = "test_client_secret_123"
	testAuthUrl      = "test_auth_url_123"
	testTokenUrl     = "test_token_url_123"
)

func TestCreateOauthConfigFromSecret(t *testing.T) {
	t.Run("all fields set ok", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				oauthCfgSecretFieldClientId:     []byte(testClientId),
				oauthCfgSecretFieldClientSecret: []byte(testClientSecret),
				oauthCfgSecretFieldAuthUrl:      []byte(testAuthUrl),
				oauthCfgSecretFieldTokenUrl:     []byte(testTokenUrl),
			},
		}

		oauthCfg := &oauth2.Config{}
		err := initializeConfigFromSecret(secret, oauthCfg)

		assert.NoError(t, err)
		assert.Equal(t, testClientId, oauthCfg.ClientID)
		assert.Equal(t, testClientSecret, oauthCfg.ClientSecret)
		assert.Equal(t, testAuthUrl, oauthCfg.Endpoint.AuthURL)
		assert.Equal(t, testTokenUrl, oauthCfg.Endpoint.TokenURL)
	})

	t.Run("error if missing client id", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				oauthCfgSecretFieldClientSecret: []byte(testClientSecret),
				oauthCfgSecretFieldAuthUrl:      []byte(testAuthUrl),
				oauthCfgSecretFieldTokenUrl:     []byte(testTokenUrl),
			},
		}

		oauthCfg := &oauth2.Config{}
		err := initializeConfigFromSecret(secret, oauthCfg)

		assert.Error(t, err)
	})

	t.Run("error if missing client secret", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				oauthCfgSecretFieldClientId: []byte(testClientId),
				oauthCfgSecretFieldAuthUrl:  []byte(testAuthUrl),
				oauthCfgSecretFieldTokenUrl: []byte(testTokenUrl),
			},
		}

		oauthCfg := &oauth2.Config{}
		err := initializeConfigFromSecret(secret, oauthCfg)

		assert.Error(t, err)
	})

	t.Run("ok with just client id and secret", func(t *testing.T) {
		secret := &v1.Secret{
			Data: map[string][]byte{
				oauthCfgSecretFieldClientId:     []byte(testClientId),
				oauthCfgSecretFieldClientSecret: []byte(testClientSecret),
			},
		}

		oauthCfg := &oauth2.Config{}
		err := initializeConfigFromSecret(secret, oauthCfg)

		assert.NoError(t, err)
		assert.Equal(t, testClientId, oauthCfg.ClientID)
		assert.Equal(t, testClientSecret, oauthCfg.ClientSecret)
		assert.Equal(t, "", oauthCfg.Endpoint.AuthURL)
		assert.Equal(t, "", oauthCfg.Endpoint.TokenURL)
	})
}

func TestFindOauthConfigSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(v1.AddToScheme(scheme))
	ctx := context.TODO()

	secretNamespace := "test-secretConfigNamespace"

	t.Run("no secrets", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects().Build()
		ctrl := commonController{
			K8sClient: cl,
		}

		found, secret, err := ctrl.findOauthConfigSecret(ctx, &oauthstate2.OAuthInfo{})
		assert.False(t, found)
		assert.Nil(t, secret)
		assert.NoError(t, err)
	})

	t.Run("secret found", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithLists(&v1.SecretList{
			Items: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "oauth-config-secret",
						Namespace: secretNamespace,
						Labels: map[string]string{
							v1beta1.ServiceProviderTypeLabel: string(config.ServiceProviderTypeGitHub),
						},
					},
				},
			},
		}).Build()
		ctrl := commonController{
			Config: config.ServiceProviderConfiguration{
				ServiceProviderType: config.ServiceProviderTypeGitHub,
			},
			K8sClient: cl,
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}

		found, secret, err := ctrl.findOauthConfigSecret(ctx, oauthState)
		assert.True(t, found)
		assert.NotNil(t, secret)
		assert.NoError(t, err)
	})

	t.Run("secret for different sp", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithLists(&v1.SecretList{
			Items: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "oauth-config-secret",
						Namespace: secretNamespace,
						Labels: map[string]string{
							v1beta1.ServiceProviderTypeLabel: string(config.ServiceProviderTypeQuay),
						},
					},
				},
			},
		}).Build()
		ctrl := commonController{
			Config: config.ServiceProviderConfiguration{
				ServiceProviderType: config.ServiceProviderTypeGitHub,
			},
			K8sClient: cl,
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}

		found, secret, err := ctrl.findOauthConfigSecret(ctx, oauthState)
		assert.False(t, found)
		assert.Nil(t, secret)
		assert.NoError(t, err)
	})

	t.Run("secret in different namespace", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithLists(&v1.SecretList{
			Items: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "oauth-config-secret",
						Namespace: "different-namespace",
						Labels: map[string]string{
							v1beta1.ServiceProviderTypeLabel: string(config.ServiceProviderTypeGitHub),
						},
					},
				},
			},
		}).Build()
		ctrl := commonController{
			Config: config.ServiceProviderConfiguration{
				ServiceProviderType: config.ServiceProviderTypeGitHub,
			},
			K8sClient: cl,
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}

		found, secret, err := ctrl.findOauthConfigSecret(ctx, oauthState)
		assert.False(t, found)
		assert.Nil(t, secret)
		assert.NoError(t, err)
	})

	t.Run("no permission for secrets", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjectTracker(&mockTracker{
			listImpl: func(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error) {
				return nil, errors.NewForbidden(schema.GroupResource{
					Group:    "test-group",
					Resource: "test-resource",
				}, "nenene", fmt.Errorf("test err"))
			}}).Build()
		ctrl := commonController{
			Config: config.ServiceProviderConfiguration{
				ServiceProviderType: config.ServiceProviderTypeGitHub,
			},
			K8sClient: cl,
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}

		found, secret, err := ctrl.findOauthConfigSecret(ctx, oauthState)
		assert.False(t, found)
		assert.Nil(t, secret)
		assert.NoError(t, err)
	})

	t.Run("error from kube", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjectTracker(&mockTracker{
			listImpl: func(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error) {
				return nil, errors.NewBadRequest("nenenene")
			}}).Build()
		ctrl := commonController{
			Config: config.ServiceProviderConfiguration{
				ServiceProviderType: config.ServiceProviderTypeGitHub,
			},
			K8sClient: cl,
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}

		found, secret, err := ctrl.findOauthConfigSecret(ctx, oauthState)
		assert.False(t, found)
		assert.Nil(t, secret)
		assert.Error(t, err)
	})
}

func TestObtainOauthConfig(t *testing.T) {
	t.Run("no secret use default oauth config", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))
		ctx := context.TODO()

		//secretNamespace := "test-secretConfigNamespace"

		cl := fake.NewClientBuilder().WithScheme(scheme).Build()

		ctrl := commonController{
			Config: config.ServiceProviderConfiguration{
				ClientId:               "eh?",
				ClientSecret:           "bleh?",
				ServiceProviderType:    config.ServiceProviderTypeGitHub,
				ServiceProviderBaseUrl: "http://bleh.eh",
			},
			K8sClient: cl,
			Endpoint:  github.Endpoint,
			BaseUrl:   "baseurl",
		}

		oauthState := &oauthstate2.OAuthInfo{}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthState)

		assert.NoError(t, err)
		assert.NotNil(t, oauthCfg)
		assert.Equal(t, oauthCfg.ClientID, "eh?")
		assert.Equal(t, oauthCfg.ClientSecret, "bleh?")
		assert.Equal(t, oauthCfg.Endpoint.AuthURL, github.Endpoint.AuthURL)
		assert.Equal(t, oauthCfg.Endpoint.TokenURL, github.Endpoint.TokenURL)
		assert.Equal(t, oauthCfg.Endpoint.AuthStyle, github.Endpoint.AuthStyle)
		assert.Contains(t, oauthCfg.RedirectURL, "baseurl")
	})

	t.Run("use oauth config from secret", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))
		ctx := context.TODO()

		secretNamespace := "test-secretConfigNamespace"

		cl := fake.NewClientBuilder().WithScheme(scheme).WithLists(&v1.SecretList{
			Items: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "oauth-config-secret",
						Namespace: secretNamespace,
						Labels: map[string]string{
							v1beta1.ServiceProviderTypeLabel: string(config.ServiceProviderTypeGitHub),
						},
					},
					Data: map[string][]byte{
						oauthCfgSecretFieldClientId:     []byte("testclientid"),
						oauthCfgSecretFieldClientSecret: []byte("testclientsecret"),
					},
				},
			},
		}).Build()

		ctrl := commonController{
			Config: config.ServiceProviderConfiguration{
				ClientId:               "eh?",
				ClientSecret:           "bleh?",
				ServiceProviderType:    config.ServiceProviderTypeGitHub,
				ServiceProviderBaseUrl: "http://bleh.eh",
			},
			K8sClient: cl,
			Endpoint:  github.Endpoint,
			BaseUrl:   "baseurl",
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthState)

		assert.NoError(t, err)
		assert.NotNil(t, oauthCfg)
		assert.Equal(t, oauthCfg.ClientID, "testclientid")
		assert.Equal(t, oauthCfg.ClientSecret, "testclientsecret")
		assert.Equal(t, oauthCfg.Endpoint.AuthURL, github.Endpoint.AuthURL)
		assert.Equal(t, oauthCfg.Endpoint.TokenURL, github.Endpoint.TokenURL)
		assert.Equal(t, oauthCfg.Endpoint.AuthStyle, github.Endpoint.AuthStyle)
		assert.Contains(t, oauthCfg.RedirectURL, "baseurl")
	})

	t.Run("found invalid oauth config secret", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))
		ctx := context.TODO()

		secretNamespace := "test-secretConfigNamespace"

		cl := fake.NewClientBuilder().WithScheme(scheme).WithLists(&v1.SecretList{
			Items: []v1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "oauth-config-secret",
						Namespace: secretNamespace,
						Labels: map[string]string{
							v1beta1.ServiceProviderTypeLabel: string(config.ServiceProviderTypeGitHub),
						},
					},
					Data: map[string][]byte{
						oauthCfgSecretFieldClientId: []byte("testclientid"),
					},
				},
			},
		}).Build()

		ctrl := commonController{
			Config: config.ServiceProviderConfiguration{
				ClientId:               "eh?",
				ClientSecret:           "bleh?",
				ServiceProviderType:    config.ServiceProviderTypeGitHub,
				ServiceProviderBaseUrl: "http://bleh.eh",
			},
			K8sClient: cl,
			Endpoint:  github.Endpoint,
			BaseUrl:   "baseurl",
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthState)

		assert.Error(t, err)
		assert.Nil(t, oauthCfg)
	})

	t.Run("error when failed kube request", func(t *testing.T) {
		scheme := runtime.NewScheme()
		utilruntime.Must(v1.AddToScheme(scheme))
		ctx := context.TODO()

		secretNamespace := "test-secretConfigNamespace"

		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjectTracker(&mockTracker{
			listImpl: func(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error) {
				return nil, errors.NewBadRequest("nenenene")
			}}).Build()

		ctrl := commonController{
			K8sClient: cl,
		}

		oauthState := &oauthstate2.OAuthInfo{
			TokenNamespace:      secretNamespace,
			ServiceProviderType: config.ServiceProviderTypeGitHub,
		}

		oauthCfg, err := ctrl.obtainOauthConfig(ctx, oauthState)

		assert.Error(t, err)
		assert.Nil(t, oauthCfg)
	})
}

type mockTracker struct {
	listImpl func(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error)
}

func (t *mockTracker) Add(obj runtime.Object) error {
	panic("not needed for now")
}

func (t *mockTracker) Get(gvr schema.GroupVersionResource, ns, name string) (runtime.Object, error) {
	panic("not needed for now")
}

func (t *mockTracker) Create(gvr schema.GroupVersionResource, obj runtime.Object, ns string) error {
	panic("not needed for now")
}

func (t *mockTracker) Update(gvr schema.GroupVersionResource, obj runtime.Object, ns string) error {
	panic("not needed for now")
}

func (t *mockTracker) List(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error) {
	return t.listImpl(gvr, gvk, ns)
}

func (t *mockTracker) Delete(gvr schema.GroupVersionResource, ns, name string) error {
	panic("not needed for now")
}

func (t *mockTracker) Watch(gvr schema.GroupVersionResource, ns string) (watch.Interface, error) {
	panic("not needed for now")
}