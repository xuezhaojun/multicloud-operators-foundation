// Licensed Materials - Property of IBM
// (c) Copyright IBM Corporation 2018. All Rights Reserved.
// Note to U.S. Government Users Restricted Rights:
// Use, duplication or disclosure restricted by GSA ADP Schedule
// Contract with IBM Corp.

// Code generated by client-gen. DO NOT EDIT.

package internalversion

import (
	"github.ibm.com/IBMPrivateCloud/multicloud-operators-foundation/pkg/client/clientset_generated/internalclientset/scheme"
	rest "k8s.io/client-go/rest"
)

type McmInterface interface {
	RESTClient() rest.Interface
	ClusterJoinRequestsGetter
	ClusterStatusesGetter
	ResourceViewsGetter
	WorksGetter
	WorkSetsGetter
}

// McmClient is used to interact with features provided by the mcm.ibm.com group.
type McmClient struct {
	restClient rest.Interface
}

func (c *McmClient) ClusterJoinRequests() ClusterJoinRequestInterface {
	return newClusterJoinRequests(c)
}

func (c *McmClient) ClusterStatuses(namespace string) ClusterStatusInterface {
	return newClusterStatuses(c, namespace)
}

func (c *McmClient) ResourceViews(namespace string) ResourceViewInterface {
	return newResourceViews(c, namespace)
}

func (c *McmClient) Works(namespace string) WorkInterface {
	return newWorks(c, namespace)
}

func (c *McmClient) WorkSets(namespace string) WorkSetInterface {
	return newWorkSets(c, namespace)
}

// NewForConfig creates a new McmClient for the given config.
func NewForConfig(c *rest.Config) (*McmClient, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &McmClient{client}, nil
}

// NewForConfigOrDie creates a new McmClient for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *McmClient {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new McmClient for the given RESTClient.
func New(c rest.Interface) *McmClient {
	return &McmClient{c}
}

func setConfigDefaults(config *rest.Config) error {
	config.APIPath = "/apis"
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	if config.GroupVersion == nil || config.GroupVersion.Group != scheme.Scheme.PrioritizedVersionsForGroup("mcm.ibm.com")[0].Group {
		gv := scheme.Scheme.PrioritizedVersionsForGroup("mcm.ibm.com")[0]
		config.GroupVersion = &gv
	}
	config.NegotiatedSerializer = scheme.Codecs

	if config.QPS == 0 {
		config.QPS = 5
	}
	if config.Burst == 0 {
		config.Burst = 10
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *McmClient) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}