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

package serviceprovider

import (
	"context"

	api "github.com/redhat-appstudio/service-provider-integration-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ ServiceProvider = (*Github)(nil)

type Github struct {
	Client client.Client
}

func (g *Github) LookupToken(ctx context.Context, binding *api.SPIAccessTokenBinding) (*api.SPIAccessToken, error) {
	// TODO implement

	// for now just return the first SPIAccessToken that we find so that we prevent infinitely many SPIAccessTokens
	// being created during the tests :)
	ats := &api.SPIAccessTokenList{}
	if err := g.Client.List(ctx, ats, client.Limit(1)); err != nil {
		return nil, err
	}

	if len(ats.Items) == 0 {
		return nil, nil
	}

	return &ats.Items[0], nil
}

func (g *Github) GetServiceProviderUrlForRepo(repoUrl string) (string, error) {
	return getHostWithScheme(repoUrl)
}