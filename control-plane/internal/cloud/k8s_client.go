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
//
// On managed Kubernetes (e.g. DigitalOcean DOKS), the in-cluster ClusterIP
// (10.x.x.x) may be unreachable due to CGNAT routing / Cilium NetworkPolicy
// issues. Set KUBERNETES_API_URL to the external API server URL to bypass:
//
//	KUBERNETES_API_URL=https://<cluster-id>.k8s.ondigitalocean.com
func NewInClusterClient() (*InClusterK8sClient, error) {
	tokenBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, fmt.Errorf("failed to read service account token: %w", err)
	}

	// Allow override for managed K8s where in-cluster ClusterIP is unreachable
	apiServer := os.Getenv("KUBERNETES_API_URL")
	if apiServer == "" {
		host := os.Getenv("KUBERNETES_SERVICE_HOST")
		port := os.Getenv("KUBERNETES_SERVICE_PORT")
		if host == "" || port == "" {
			return nil, fmt.Errorf("not running inside Kubernetes (KUBERNETES_SERVICE_HOST/PORT not set)")
		}
		apiServer = fmt.Sprintf("https://%s:%s", host, port)
	}

	// Use in-cluster CA cert if available; for external URLs, fall back to
	// system cert pool (DO's *.k8s.ondigitalocean.com uses a public CA).
	tlsCfg := tlsConfigFromServiceAccount()

	logger.Logger.Info().
		Str("api_server", apiServer).
		Msg("K8s client initialized")

	return &InClusterK8sClient{
		apiServer: apiServer,
		token:     string(tokenBytes),
		client: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: tlsCfg,
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

	logger.Logger.Debug().
		Str("url", createURL).
		Str("pod", spec.Name).
		Str("namespace", spec.Namespace).
		Msg("creating agent pod via K8s API")

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Logger.Error().
			Err(err).
			Str("url", createURL).
			Str("api_server", c.apiServer).
			Msg("K8s API request failed — check network connectivity and NetworkPolicies")
		return nil, fmt.Errorf("K8s API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		logger.Logger.Error().
			Int("status", resp.StatusCode).
			Str("body", string(respBody)).
			Msg("K8s API returned error")
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
		"name":            "agent",
		"image":           spec.Image,
		"imagePullPolicy": "Always",
		"env":             envVars,
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
		// In node mode the agent connects to the central gateway via WebSocket
		// and doesn't expose an HTTP server. Use exec probes to check process health.
		"livenessProbe": map[string]interface{}{
			"exec": map[string]interface{}{
				"command": []string{"sh", "-c", "kill -0 1"},
			},
			"initialDelaySeconds": 10,
			"periodSeconds":       20,
			"timeoutSeconds":      3,
			"failureThreshold":    3,
		},
		"startupProbe": map[string]interface{}{
			"exec": map[string]interface{}{
				"command": []string{"sh", "-c", "kill -0 1"},
			},
			"initialDelaySeconds": 2,
			"periodSeconds":       5,
			"timeoutSeconds":      3,
			"failureThreshold":    30,
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
			"name":            sc.Name,
			"image":           sc.Image,
			"imagePullPolicy": "Always",
			"env":             scEnv,
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
		// Attach lifecycle hooks if specified
		if len(sc.PostStart) > 0 {
			scContainer["lifecycle"] = map[string]interface{}{
				"postStart": map[string]interface{}{
					"exec": map[string]interface{}{
						"command": sc.PostStart,
					},
				},
			}
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

// tlsConfigFromServiceAccount builds TLS config using the in-cluster CA cert
// combined with system certs. This handles both in-cluster ClusterIP
// (needs the K8s CA) and external API URLs (need public CAs).
func tlsConfigFromServiceAccount() *tls.Config {
	// Start with system cert pool for external URLs
	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	// Add in-cluster CA cert if available
	caCert, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		logger.Logger.Warn().Err(err).Msg("failed to read in-cluster CA cert, using system certs only")
	} else {
		rootCAs.AppendCertsFromPEM(caCert)
	}

	return &tls.Config{
		RootCAs: rootCAs,
	}
}
