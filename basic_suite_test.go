// Copyright (c) 2020 Doc.ai and/or its affiliates.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration_k8s_kind_test

import (
	"testing"

	"github.com/networkservicemesh/integration-k8s-kind/k8s"

	v1 "k8s.io/api/apps/v1"
	v1core "k8s.io/api/core/v1"

	"github.com/networkservicemesh/integration-k8s-kind/spire"

	"github.com/edwarnicke/exechelper"
	"github.com/sirupsen/logrus"

	"github.com/stretchr/testify/suite"
)

type BasicTestsSuite struct {
	suite.Suite
	options []*exechelper.Option
}

func (s *BasicTestsSuite) SetupSuite() {
	writer := logrus.StandardLogger().Writer()

	s.options = []*exechelper.Option{
		exechelper.WithStderr(writer),
		exechelper.WithStdout(writer),
	}

	s.Require().NoError(spire.Setup(s.options...))

	s.Require().NoError(k8s.KindRequire("fake-nsmgr", "nse"))
}

func (s *BasicTestsSuite) TearDownSuite() {
	s.Require().NoError(spire.Delete(s.options...))
}

func (s *BasicTestsSuite) TearDownTest() {
	k8s.ShowLogs(k8s.DefaultNamespace, s.options...)

	s.Require().NoError(exechelper.Run("kubectl delete serviceaccounts --all"))
	s.Require().NoError(exechelper.Run("kubectl delete services --all"))
	s.Require().NoError(exechelper.Run("kubectl delete deployment --all"))
	s.Require().NoError(exechelper.Run("kubectl delete pods --all --grace-period=0 --force"))
}

func (s *BasicTestsSuite) TestNSE_CanRegisterInRegistry() {
	defer k8s.NoRestarts(s.T())

	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/registry-service.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/registry-memory.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl wait --timeout=120s  --for=condition=ready pod -l app=nsm-registry", s.options...))

	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/nse.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl wait --timeout=120s  --for=condition=ready pod -l app=nse", s.options...))
}

func (s *BasicTestsSuite) TestNSMgr_CanCanFindNSEInRegistry() {
	defer k8s.NoRestarts(s.T())

	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/registry-service.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/registry-memory.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl wait --timeout=120s  --for=condition=ready pod -l app=nsm-registry", s.options...))

	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/nse.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl wait --timeout=120s  --for=condition=ready pod -l app=nse", s.options...))

	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/fake-nsmgr.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl wait --timeout=120s  --for=condition=ready pod -l app=fake-nsmgr", s.options...))
}

func (s *BasicTestsSuite) TestProxyRegistryDNS_CanCanFindNSE_Local() {
	defer k8s.NoRestarts(s.T())

	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/proxy-registry-service.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/registry-service.yaml", s.options...))

	s.Require().NoError(k8s.ApplyDeployment("./deployments/registry-proxy-dns.yaml", func(proxyRegistry *v1.Deployment) {
		proxyRegistry.Spec.Template.Spec.Containers[0].Env = append(proxyRegistry.Spec.Template.Spec.Containers[0].Env, v1core.EnvVar{
			Name:  "REGISTRY-PROXY-DNS_DOMAIN",
			Value: "default.svc.cluster.local",
		})
	}))

	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/registry-memory.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl wait --timeout=120s  --for=condition=ready pod -l app=nsm-registry", s.options...))

	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/nse.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl wait --timeout=120s  --for=condition=ready pod -l app=nse", s.options...))

	s.Require().NoError(exechelper.Run("kubectl wait --timeout=120s  --for=condition=ready pod -l app=nsm-registry-proxy-dns", s.options...))

	s.Require().NoError(k8s.ApplyDeployment("./deployments/fake-nsmgr.yaml", func(nsmgr *v1.Deployment) {
		nsmgr.Spec.Template.Spec.Containers[0].Env = append(nsmgr.Spec.Template.Spec.Containers[0].Env, v1core.EnvVar{
			Name:  "FAKE-NSMGR_FIND_NETWORK_SERVICE_ENDPOINT_NAME",
			Value: "icmp-responder@default.svc.cluster.local",
		})
	}))

	s.Require().NoError(exechelper.Run("kubectl wait --timeout=120s  --for=condition=ready pod -l app=fake-nsmgr", s.options...))
}

func (s *BasicTestsSuite) TestDeployMemoryRegistry() {
	defer k8s.NoRestarts(s.T())

	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/registry-service.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/registry-memory.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl wait --timeout=120s  --for=condition=ready pod -l app=nsm-registry", s.options...))
	s.Require().NoError(exechelper.Run("kubectl describe pod -l app=nsm-registry", s.options...))
}

func (s *BasicTestsSuite) TestDeployAlpine() {
	defer k8s.NoRestarts(s.T())

	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/registry-service.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/alpine.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl wait --for=condition=ready pod -l app=alpine", s.options...))
}

func (s *BasicTestsSuite) TestNsmgr() {
	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/nsmgr/nsmgr-namespace.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl apply -f ./deployments/nsmgr/nsmgr.yaml", s.options...))
	s.Require().NoError(exechelper.Run("kubectl wait --for=condition=ready pod -l app=nsmgr --namespace nsmgr", s.options...))

	podInfo, err := k8s.DescribePod("nsmgr", "", map[string]string{"app": "nsmgr"})
	s.Require().NoError(err)
	s.Require().Equal(v1core.PodRunning, podInfo.Status.Phase)

	s.Require().NoError(exechelper.Run("kubectl delete -f ./deployments/nsmgr/nsmgr.yaml --grace-period=0 --force", s.options...))
}

func TestRunBasicSuite(t *testing.T) {
	suite.Run(t, &BasicTestsSuite{})
}
