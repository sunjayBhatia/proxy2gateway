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

package main

import (
	"fmt"
	"io"
	"os"

	contour_api_v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/projectcontour/proxy2gateway/internal/translate"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	apimachinery_util_yaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/printers"
	gatewayapi_v1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const (
	build = "dev"
)

func main() {
	log := logrus.StandardLogger()
	app := kingpin.New("proxy2gateway", "Contour HTTPProxy to Gateway API resource conversion tool.")
	app.Version(build)
	httpProxiesPath := app.Flag("http-proxies", "YAML file to parse for HTTPProxy objects").Required().ExistingFile()
	baseGatewayPath := app.Flag("base-gateway", "YAML file to parse for base Gateway resource").ExistingFile()

	kingpin.MustParse(app.Parse(os.Args[1:]))

	httpProxiesFile, err := os.Open(*httpProxiesPath)
	if err != nil {
		log.WithField("error", err).Fatal("could not open HTTPProxy yaml file")
	}
	defer httpProxiesFile.Close()

	httpProxyDecoder := apimachinery_util_yaml.NewYAMLToJSONDecoder(httpProxiesFile)

	var proxies []*contour_api_v1.HTTPProxy
	for {
		proxy := &contour_api_v1.HTTPProxy{}
		if err := httpProxyDecoder.Decode(proxy); err != nil {
			if err == io.EOF {
				// We're done collecting HTTPProxies.
				break
			}
			log.WithField("error", err).Fatal("error decoding HTTPProxy yaml")

		}
		proxies = append(proxies, proxy)
	}

	var baseGateway *gatewayapi_v1alpha2.Gateway
	if baseGatewayPath != nil && len(*baseGatewayPath) > 0 {
		baseGatewayFile, err := os.Open(*baseGatewayPath)
		if err != nil {
			log.WithField("error", err).Fatal("could not open Gateway yaml file")
		}
		defer baseGatewayFile.Close()

		gatewayDecoder := apimachinery_util_yaml.NewYAMLToJSONDecoder(baseGatewayFile)

		baseGateway = &gatewayapi_v1alpha2.Gateway{}
		err = gatewayDecoder.Decode(baseGateway)
		if err != nil {
			log.WithField("error", err).Fatal("error decoding HTTPProxy yaml")
		}
	}

	outGW, outHTTPRoutes, outTLSRoutes, err := translate.HTTPProxiesToGatewayResources(log, baseGateway, proxies)
	if err != nil {
		log.WithField("error", err).Fatal("failed to translate proxies to Gateway API resources")
	}

	printer := printers.YAMLPrinter{}

	if outGW != nil {
		fmt.Println("---")
		if err := printer.PrintObj(outGW, os.Stdout); err != nil {
			log.WithField("error", err).Fatal("error printing Gateway")
		}
	}

	for _, route := range outHTTPRoutes {
		fmt.Println("---")
		if err := printer.PrintObj(route, os.Stdout); err != nil {
			log.WithField("error", err).Fatal("error printing HTTPRoute")
		}
	}
	for _, route := range outTLSRoutes {
		fmt.Println("---")
		if err := printer.PrintObj(route, os.Stdout); err != nil {
			log.WithField("error", err).Fatal("error printing TLSRoute")
		}
	}
}
