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

package translate

import (
	"fmt"

	contour_api_v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/sirupsen/logrus"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	gatewayapi_v1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func HTTPProxiesToGatewayResources(log logrus.FieldLogger, baseGateway *gatewayapi_v1alpha2.Gateway, proxies []*contour_api_v1.HTTPProxy) (*gatewayapi_v1alpha2.Gateway, []*gatewayapi_v1alpha2.HTTPRoute, []*gatewayapi_v1alpha2.TLSRoute, error) {
	var (
		httpRoutes      []*gatewayapi_v1alpha2.HTTPRoute
		tlsRoutes       []*gatewayapi_v1alpha2.TLSRoute
		outGateway      *gatewayapi_v1alpha2.Gateway
		commonRouteSpec gatewayapi_v1alpha2.CommonRouteSpec
	)
	if baseGateway != nil {
		outGateway = baseGateway.DeepCopy()
	}
	commonRouteSpec = commonRouteSpecFromGateway(baseGateway)

	for _, proxy := range proxies {
		objectMeta := meta_v1.ObjectMeta{
			Name:      proxy.Name,
			Namespace: proxy.Namespace,
		}
		log := log.WithFields(logrus.Fields{"name": proxy.Name, "namespace": proxy.Namespace})

		if len(proxy.Spec.Includes) > 0 {
			log.Warn("skipping HTTPProxy: translating includes not supported")
			continue
		}

		// We should build a TLSRoute.
		if proxy.Spec.TCPProxy != nil {
			var backendRefs []gatewayapi_v1alpha2.BackendRef
			for _, s := range proxy.Spec.TCPProxy.Services {
				backendRefs = append(backendRefs, gatewayapi_v1alpha2.BackendRef{
					BackendObjectReference: gatewayapi_v1alpha2.BackendObjectReference{
						Name: gatewayapi_v1alpha2.ObjectName(s.Name),
						Port: portNumPtr(s.Port),
					},
				})
			}
			route := &gatewayapi_v1alpha2.TLSRoute{
				TypeMeta: meta_v1.TypeMeta{
					Kind:       "TLSRoute",
					APIVersion: gatewayapi_v1alpha2.GroupVersion.String(),
				},
				ObjectMeta: objectMeta,
				Spec: gatewayapi_v1alpha2.TLSRouteSpec{
					CommonRouteSpec: commonRouteSpec,
					Rules: []gatewayapi_v1alpha2.TLSRouteRule{
						{BackendRefs: backendRefs},
					},
				},
			}

			tlsRoutes = append(tlsRoutes, route)

			outGateway.Spec.Listeners = addListener(outGateway.Spec.Listeners, gatewayapi_v1alpha2.Listener{
				Name:     gatewayapi_v1alpha2.SectionName(fmt.Sprintf("tls-%d", len(outGateway.Spec.Listeners))),
				Port:     gatewayapi_v1alpha2.PortNumber(443),
				Protocol: gatewayapi_v1alpha2.TLSProtocolType,
				Hostname: listenerHostnamePtr(proxy.Spec.VirtualHost.Fqdn),
				TLS: &gatewayapi_v1alpha2.GatewayTLSConfig{
					Mode: tlsModeTypePtr(gatewayapi_v1alpha2.TLSModeTerminate),
					CertificateRefs: []*gatewayapi_v1alpha2.SecretObjectReference{
						{
							Name: gatewayapi_v1alpha2.ObjectName(proxy.Spec.VirtualHost.TLS.SecretName),
						},
					},
				},
			})
		} else {
			var rules []gatewayapi_v1alpha2.HTTPRouteRule
			for _, r := range proxy.Spec.Routes {
				var match gatewayapi_v1alpha2.HTTPRouteMatch
				for _, c := range r.Conditions {
					if c.Prefix != "" {
						match.Path = &gatewayapi_v1alpha2.HTTPPathMatch{
							Type:  pathMatchTypePtr(gatewayapi_v1alpha2.PathMatchPathPrefix),
							Value: pointer.String(c.Prefix),
						}
					}
				}

				var backendRefs []gatewayapi_v1alpha2.HTTPBackendRef
				for _, s := range r.Services {
					backendRefs = append(backendRefs, gatewayapi_v1alpha2.HTTPBackendRef{
						BackendRef: gatewayapi_v1alpha2.BackendRef{
							BackendObjectReference: gatewayapi_v1alpha2.BackendObjectReference{
								Name: gatewayapi_v1alpha2.ObjectName(s.Name),
								Port: portNumPtr(s.Port),
							},
						},
					})
				}

				rules = append(rules, gatewayapi_v1alpha2.HTTPRouteRule{
					Matches:     []gatewayapi_v1alpha2.HTTPRouteMatch{match},
					BackendRefs: backendRefs,
				})
			}

			route := &gatewayapi_v1alpha2.HTTPRoute{
				TypeMeta: meta_v1.TypeMeta{
					Kind:       "HTTPRoute",
					APIVersion: gatewayapi_v1alpha2.GroupVersion.String(),
				},
				ObjectMeta: objectMeta,
				Spec: gatewayapi_v1alpha2.HTTPRouteSpec{
					CommonRouteSpec: commonRouteSpec,
					Rules:           rules,
				},
			}

			httpRoutes = append(httpRoutes, route)

			outGateway.Spec.Listeners = addListener(outGateway.Spec.Listeners, gatewayapi_v1alpha2.Listener{
				Name:     gatewayapi_v1alpha2.SectionName(fmt.Sprintf("http-%d", len(outGateway.Spec.Listeners))),
				Port:     gatewayapi_v1alpha2.PortNumber(80),
				Protocol: gatewayapi_v1alpha2.HTTPProtocolType,
				Hostname: listenerHostnamePtr(proxy.Spec.VirtualHost.Fqdn),
			})
		}
	}
	return outGateway, httpRoutes, tlsRoutes, nil
}

func pathMatchTypePtr(val gatewayapi_v1alpha2.PathMatchType) *gatewayapi_v1alpha2.PathMatchType {
	return &val
}

func portNumPtr(port int) *gatewayapi_v1alpha2.PortNumber {
	pn := gatewayapi_v1alpha2.PortNumber(port)
	return &pn
}

func groupPtr(group string) *gatewayapi_v1alpha2.Group {
	gwGroup := gatewayapi_v1alpha2.Group(group)
	return &gwGroup
}

func kindPtr(kind string) *gatewayapi_v1alpha2.Kind {
	gwKind := gatewayapi_v1alpha2.Kind(kind)
	return &gwKind
}

func namespacePtr(namespace string) *gatewayapi_v1alpha2.Namespace {
	gwNamespace := gatewayapi_v1alpha2.Namespace(namespace)
	return &gwNamespace
}

func tlsModeTypePtr(mode gatewayapi_v1alpha2.TLSModeType) *gatewayapi_v1alpha2.TLSModeType {
	return &mode
}

func listenerHostnamePtr(host string) *gatewayapi_v1alpha2.Hostname {
	h := gatewayapi_v1alpha2.Hostname(host)
	return &h
}

func commonRouteSpecFromGateway(gw *gatewayapi_v1alpha2.Gateway) gatewayapi_v1alpha2.CommonRouteSpec {
	crs := gatewayapi_v1alpha2.CommonRouteSpec{}
	if gw != nil {

		parentRef := gatewayapi_v1alpha2.ParentRef{
			Group: groupPtr(gatewayapi_v1alpha2.GroupName),
			Kind:  kindPtr("Gateway"),
			Name:  gatewayapi_v1alpha2.ObjectName(gw.Name),
		}

		if gw.Namespace != "" {
			parentRef.Namespace = namespacePtr(gw.Namespace)
		}

		crs.ParentRefs = []gatewayapi_v1alpha2.ParentRef{parentRef}
	}

	return crs
}

// Adds a listener if no existing listener matches the requirements for the HTTPProxy.
func addListener(existingListeners []gatewayapi_v1alpha2.Listener, newListener gatewayapi_v1alpha2.Listener) []gatewayapi_v1alpha2.Listener {
	for _, l := range existingListeners {
		protocolsMatch := l.Protocol == newListener.Protocol
		portsMatch := l.Port == newListener.Port
		hostnamesMatch := (l.Hostname == nil && newListener.Hostname == nil) ||
			(l.Hostname != nil && newListener.Hostname != nil && *l.Hostname == *newListener.Hostname)
		// TODO: this is incomplete, need to assess if the TLS fields match
		if protocolsMatch && portsMatch && hostnamesMatch {
			return existingListeners
		}
	}
	newListener.Name = listenerName(newListener.Protocol, len(existingListeners))
	return append(existingListeners, newListener)
}

func listenerName(protocol gatewayapi_v1alpha2.ProtocolType, index int) gatewayapi_v1alpha2.SectionName {
	var prefix string
	switch protocol {
	case gatewayapi_v1alpha2.HTTPProtocolType:
		prefix = `http-%d`
	case gatewayapi_v1alpha2.HTTPSProtocolType:
		prefix = `https-%d`
	case gatewayapi_v1alpha2.TLSProtocolType:
		prefix = `tls-%d`
	}
	return gatewayapi_v1alpha2.SectionName(fmt.Sprintf(prefix, index))
}
