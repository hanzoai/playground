package cloud

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hanzoai/playground/control-plane/internal/logger"
)

// InClusterK8sClient talks to the Kubernetes API using the service account
// token mounted in-pod. No client-go dependency — uses raw HTTP to the
// K8s API server, keeping the binary lean.
type InClusterK8sClient struct {
	apiServer string
	token     string
	client    *http.Client
}

// NewInClusterClient creates a K8s client using the in-cluster service account.
func NewInClusterClient() (*InClusterK8sClient, error) {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, fmt.Errorf("not running inside Kubernetes (KUBERNETES_SERVICE_HOST/PORT not set)")
	}

	tokenBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, fmt.Errorf("failed to read service account token: %w", err)
	}

	return &InClusterK8sClient{
		apiServer: fmt.Sprintf("https://%s:%s", host, port),
		token:     string(tokenBytes),
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: tlsConfigFromServiceAccount(),
			},
		},
	}, nil
}

// CreateAgentPod creates a pod via the K8s API.
func (c *InClusterK8sClient) CreateAgentPod(ctx context.Context, spec *PodSpec) (*PodStatus, error) {
	pod := buildPodManifest(spec)

	body, err := json.Marshal(pod)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pod spec: %w", err)
	}

	createURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods", c.apiServer, spec.Namespace)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("K8s API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("K8s API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode K8s response: %w", err)
	}

	return &PodStatus{
		Name:      spec.Name,
		Namespace: spec.Namespace,
		Phase:     "Pending",
		Ready:     false,
	}, nil
}

// DeleteAgentPod deletes a pod.
func (c *InClusterK8sClient) DeleteAgentPod(ctx context.Context, namespace, podName string, gracePeriod time.Duration) error {
	deleteURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s?gracePeriodSeconds=%d",
		c.apiServer, namespace, podName, int64(gracePeriod.Seconds()))

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("K8s delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil // Already gone
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("K8s API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetNodePod gets the status of a pod.
func (c *InClusterK8sClient) GetNodePod(ctx context.Context, namespace, podName string) (*PodStatus, error) {
	getURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s", c.apiServer, namespace, podName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("K8s get request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("pod %s/%s not found", namespace, podName)
	}

	var pod map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&pod); err != nil {
		return nil, fmt.Errorf("failed to decode pod: %w", err)
	}

	return extractPodStatus(pod)
}

// ListAgentPods lists pods matching a label selector.
func (c *InClusterK8sClient) ListAgentPods(ctx context.Context, namespace, labelSelector string) ([]*PodStatus, error) {
	listURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods?labelSelector=%s",
		c.apiServer, namespace, url.QueryEscape(labelSelector))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("K8s list request failed: %w", err)
	}
	defer resp.Body.Close()

	var podList map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&podList); err != nil {
		return nil, fmt.Errorf("failed to decode pod list: %w", err)
	}

	items, ok := podList["items"].([]interface{})
	if !ok {
		return nil, nil
	}

	var statuses []*PodStatus
	for _, item := range items {
		if podMap, ok := item.(map[string]interface{}); ok {
			ps, err := extractPodStatus(podMap)
			if err == nil {
				statuses = append(statuses, ps)
			}
		}
	}

	return statuses, nil
}

// GetPodLogs returns recent logs for a pod.
func (c *InClusterK8sClient) GetPodLogs(ctx context.Context, namespace, podName string, tailLines int64) (string, error) {
	logsURL := fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/log?tailLines=%d",
		c.apiServer, namespace, podName, tailLines)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, logsURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("K8s logs request failed: %w", err)
	}
	defer resp.Body.Close()

	logBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return "", err
	}

	return string(logBytes), nil
}

// buildPodManifest creates a K8s Pod JSON manifest from a PodSpec.
func buildPodManifest(spec *PodSpec) map[string]interface{} {
	envVars := make([]map[string]string, 0, len(spec.Env))
	for k, v := range spec.Env {
		envVars = append(envVars, map[string]string{"name": k, "value": v})
	}

	container := map[string]interface{}{
		"name":  "agent",
		"image": spec.Image,
		"env":   envVars,
		"resources": map[string]interface{}{
			"requests": map[string]string{
				"cpu":    spec.CPU,
				"memory": spec.Memory,
			},
			"limits": map[string]string{
				"cpu":    spec.LimitCPU,
				"memory": spec.LimitMemory,
			},
		},
		"ports": []map[string]interface{}{
			{"containerPort": 18789, "name": "agent", "protocol": "TCP"},
		},
		// Readiness probe: agent is ready to accept traffic
		"readinessProbe": map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": "/health",
				"port": 18789,
			},
			"initialDelaySeconds": 5,
			"periodSeconds":       10,
			"timeoutSeconds":      3,
			"failureThreshold":    3,
		},
		// Liveness probe: agent process is alive
		"livenessProbe": map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": "/health",
				"port": 18789,
			},
			"initialDelaySeconds": 15,
			"periodSeconds":       20,
			"timeoutSeconds":      5,
			"failureThreshold":    3,
		},
		// Startup probe: give agent time to initialize on first boot
		"startupProbe": map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": "/health",
				"port": 18789,
			},
			"initialDelaySeconds": 2,
			"periodSeconds":       5,
			"timeoutSeconds":      3,
			"failureThreshold":    30, // up to 150s for cold start
		},
	}

	if len(spec.Args) > 0 {
		container["args"] = spec.Args
	}

	containers := []interface{}{container}

	// Add sidecar containers (e.g. operative desktop)
	for _, sc := range spec.Sidecars {
		scEnv := make([]map[string]string, 0, len(sc.Env))
		for k, v := range sc.Env {
			scEnv = append(scEnv, map[string]string{"name": k, "value": v})
		}
		scPorts := make([]map[string]interface{}, 0, len(sc.Ports))
		for _, p := range sc.Ports {
			scPorts = append(scPorts, map[string]interface{}{
				"containerPort": p,
				"protocol":      "TCP",
			})
		}
		scContainer := map[string]interface{}{
			"name":  sc.Name,
			"image": sc.Image,
			"env":   scEnv,
			"ports": scPorts,
			"resources": map[string]interface{}{
				"requests": map[string]string{
					"cpu":    sc.CPU,
					"memory": sc.Memory,
				},
				"limits": map[string]string{
					"cpu":    sc.LimitCPU,
					"memory": sc.LimitMem,
				},
			},
		}
		containers = append(containers, scContainer)
	}

	podSpec := map[string]interface{}{
		"containers":    containers,
		"restartPolicy": "Always",
	}

	if spec.ServiceAccount != "" {
		podSpec["serviceAccountName"] = spec.ServiceAccount
	}

	if spec.ImagePullSecret != "" {
		podSpec["imagePullSecrets"] = []map[string]string{
			{"name": spec.ImagePullSecret},
		}
	}

	if len(spec.NodeSelector) > 0 {
		podSpec["nodeSelector"] = spec.NodeSelector
	}

	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":        spec.Name,
			"namespace":   spec.Namespace,
			"labels":      spec.Labels,
			"annotations": spec.Annotations,
		},
		"spec": podSpec,
	}

	return pod
}

// extractPodStatus extracts PodStatus from a raw K8s pod JSON.
func extractPodStatus(pod map[string]interface{}) (*PodStatus, error) {
	metadata, _ := pod["metadata"].(map[string]interface{})
	status, _ := pod["status"].(map[string]interface{})

	name := ""
	namespace := ""
	if metadata != nil {
		if n, ok := metadata["name"].(string); ok {
			name = n
		}
		if ns, ok := metadata["namespace"].(string); ok {
			namespace = ns
		}
	}

	phase := "Unknown"
	podIP := ""
	ready := false

	if status != nil {
		if p, ok := status["phase"].(string); ok {
			phase = p
		}
		if ip, ok := status["podIP"].(string); ok {
			podIP = ip
		}
		// Check container readiness
		if conditions, ok := status["conditions"].([]interface{}); ok {
			for _, cond := range conditions {
				if cm, ok := cond.(map[string]interface{}); ok {
					if cm["type"] == "Ready" && cm["status"] == "True" {
						ready = true
					}
				}
			}
		}
	}

	return &PodStatus{
		Name:      name,
		Namespace: namespace,
		Phase:     phase,
		Ready:     ready,
		IP:        podIP,
	}, nil
}

// tlsConfigFromServiceAccount builds TLS config using the in-cluster CA cert.
func tlsConfigFromServiceAccount() *tls.Config {
	caCert, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		logger.Logger.Warn().Err(err).Msg("failed to read in-cluster CA cert, using insecure")
		return &tls.Config{InsecureSkipVerify: true}
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	return &tls.Config{
		RootCAs: caCertPool,
	}
}
