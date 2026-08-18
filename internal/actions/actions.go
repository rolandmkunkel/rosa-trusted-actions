package actions

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/openshift-online/rosa-trusted-actions/internal/backplane"
)

type ResourceTarget struct {
	Group         string `json:"group"`
	Version       string `json:"version"`
	Resource      string `json:"resource"`
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	ClusterScoped bool   `json:"clusterScoped"`
}

type ActionRequest struct {
	Target         ResourceTarget
	ClusterVersion string
	Params         map[string]string
	PodExecutor    backplane.PodExecutor
}

type ActionResult struct {
	Resources []unstructured.Unstructured
	Message   string
}

type Action interface {
	Name() string
	RequiredRBAC(target ResourceTarget) []backplane.RBACRule
	Execute(ctx context.Context, client dynamic.Interface, req ActionRequest) (*ActionResult, error)
}

// PodExecAction is implemented by actions that need pod exec access
// (e.g. querying Prometheus via curl inside the container). The executor
// only creates a pod executor session when the action implements this interface.
type PodExecAction interface {
	Action
	UsesPodExec() bool
}

func resourceClient(client dynamic.Interface, gvr schema.GroupVersionResource, target ResourceTarget) (dynamic.ResourceInterface, error) {
	if target.ClusterScoped {
		if target.Namespace != "" {
			return nil, fmt.Errorf("namespace must be empty for cluster-scoped resource %s", target.Resource)
		}
		return client.Resource(gvr), nil
	}
	if target.Namespace == "" {
		return nil, fmt.Errorf("namespace is required for namespaced resource %s", target.Resource)
	}
	return client.Resource(gvr).Namespace(target.Namespace), nil
}

func scopeLabel(namespace string) string {
	if namespace == "" {
		return "cluster scope"
	}
	return namespace
}

func setIfPresent(m map[string]interface{}, key string, value interface{}) {
	if value != nil {
		m[key] = value
	}
}
