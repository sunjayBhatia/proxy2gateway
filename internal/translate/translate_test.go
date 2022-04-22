// Copyright Project Contour Authors
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

package translate_test

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	contour_api_v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/projectcontour/proxy2gateway/internal/translate"
	logrus_test "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apimachinery_util_yaml "k8s.io/apimachinery/pkg/util/yaml"
	gatewayapi_v1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func TestHTTPProxiesToGatewayResources(t *testing.T) {
	log, _ := logrus_test.NewNullLogger()
	for name, tc := range buildTestCases(t) {
		t.Run(name, func(t *testing.T) {
			outGateway, outHTTPRoutes, outTLSRoutes, err := translate.HTTPProxiesToGatewayResources(log, tc.inGateway, tc.inProxies)
			if tc.err != "" {
				assert.ErrorContains(t, err, tc.err)
			} else {
				assert.NoError(t, err)
				if tc.outGateway != nil {
					diff := cmp.Diff(tc.outGateway, outGateway)
					require.Empty(t, diff)
				}
				diff := cmp.Diff(tc.outHTTPRoutes, outHTTPRoutes, cmpopts.SortSlices(func(x *gatewayapi_v1alpha2.HTTPRoute, y *gatewayapi_v1alpha2.HTTPRoute) bool {
					return namespacedName(x) < namespacedName(y)
				}))
				assert.Empty(t, diff)
				diff = cmp.Diff(tc.outTLSRoutes, outTLSRoutes, cmpopts.SortSlices(func(x *gatewayapi_v1alpha2.TLSRoute, y *gatewayapi_v1alpha2.TLSRoute) bool {
					return namespacedName(x) < namespacedName(y)
				}))
				assert.Empty(t, diff)
			}
		})
	}

}

type testCase struct {
	inGateway     *gatewayapi_v1alpha2.Gateway
	outGateway    *gatewayapi_v1alpha2.Gateway
	inProxies     []*contour_api_v1.HTTPProxy
	outHTTPRoutes []*gatewayapi_v1alpha2.HTTPRoute
	outTLSRoutes  []*gatewayapi_v1alpha2.TLSRoute
	err           string
}

func buildTestCases(t *testing.T) map[string]testCase {
	testdataFiles, err := ioutil.ReadDir("testdata")
	require.NoError(t, err)

	testCases := make(map[string]testCase)

	for _, fileInfo := range testdataFiles {
		if !fileInfo.IsDir() {
			continue
		}

		var (
			inGateway  *gatewayapi_v1alpha2.Gateway
			outGateway *gatewayapi_v1alpha2.Gateway
			proxies    []*contour_api_v1.HTTPProxy
			httpRoutes []*gatewayapi_v1alpha2.HTTPRoute
			tlsRoutes  []*gatewayapi_v1alpha2.TLSRoute
		)
		testCaseDirPath := filepath.Join("testdata", fileInfo.Name())

		inGWFile, err := os.Open(filepath.Join(testCaseDirPath, "in-gateway.yaml"))
		require.NoError(t, err)
		defer inGWFile.Close()
		decoder := apimachinery_util_yaml.NewYAMLToJSONDecoder(inGWFile)
		inGateway = new(gatewayapi_v1alpha2.Gateway)
		if err := decoder.Decode(inGateway); err != nil {
			if err == io.EOF {
				// Base Gateway not provided.
				inGateway = nil
				err = nil
			}
			require.NoError(t, err)
		}

		outGWFile, err := os.Open(filepath.Join(testCaseDirPath, "out-gateway.yaml"))
		require.NoError(t, err)
		defer outGWFile.Close()
		decoder = apimachinery_util_yaml.NewYAMLToJSONDecoder(outGWFile)
		outGateway = new(gatewayapi_v1alpha2.Gateway)
		if err := decoder.Decode(outGateway); err != nil {
			if err == io.EOF {
				// Out Gateway not needed.
				outGateway = nil
				err = nil
			}
			require.NoError(t, err)
		}

		proxiesFile, err := os.Open(filepath.Join(testCaseDirPath, "in-proxies.yaml"))
		require.NoError(t, err)
		defer proxiesFile.Close()
		decoder = apimachinery_util_yaml.NewYAMLToJSONDecoder(proxiesFile)
		for {
			proxy := &contour_api_v1.HTTPProxy{}
			if err := decoder.Decode(proxy); err != nil {
				if err == io.EOF {
					// We're done collecting HTTPProxies.
					break
				}
				t.Error(err)
			}
			proxies = append(proxies, proxy)
		}

		httpRouteFile, err := os.Open(filepath.Join(testCaseDirPath, "out-httproutes.yaml"))
		require.NoError(t, err)
		defer httpRouteFile.Close()
		decoder = apimachinery_util_yaml.NewYAMLToJSONDecoder(httpRouteFile)
		for {
			route := &gatewayapi_v1alpha2.HTTPRoute{}
			if err := decoder.Decode(route); err != nil {
				if err == io.EOF {
					// We're done collecting routes.
					break
				}
				t.Error(err)
			}
			httpRoutes = append(httpRoutes, route)
		}

		tlsRouteFile, err := os.Open(filepath.Join(testCaseDirPath, "out-tlsroutes.yaml"))
		require.NoError(t, err)
		defer tlsRouteFile.Close()
		decoder = apimachinery_util_yaml.NewYAMLToJSONDecoder(tlsRouteFile)
		for {
			route := &gatewayapi_v1alpha2.TLSRoute{}
			if err := decoder.Decode(route); err != nil {
				if err == io.EOF {
					// We're done collecting routes.
					break
				}
				t.Error(err)
			}
			tlsRoutes = append(tlsRoutes, route)
		}

		errData, err := ioutil.ReadFile(filepath.Join(testCaseDirPath, "error.txt"))
		require.NoError(t, err)

		testCases[fileInfo.Name()] = testCase{
			inGateway:     inGateway,
			outGateway:    outGateway,
			inProxies:     proxies,
			outHTTPRoutes: httpRoutes,
			outTLSRoutes:  tlsRoutes,
			err:           string(errData),
		}
	}

	return testCases
}

func namespacedName(obj meta_v1.Object) string {
	return types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}.String()
}
