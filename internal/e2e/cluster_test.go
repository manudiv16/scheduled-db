package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type healthResponse struct {
	Status string `json:"status"`
	Role   string `json:"role"`
	Leader string `json:"leader,omitempty"`
	NodeID string `json:"node_id"`
}

type clusterDebugResponse struct {
	NodeID    string              `json:"node_id"`
	IsLeader  bool                `json:"is_leader"`
	Leader    string              `json:"leader"`
	RaftState string              `json:"raft_state"`
	Servers   []map[string]string `json:"servers"`
	JobCount  int                 `json:"job_count"`
}

type jobResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
}

func getAPIBase() string {
	if base := os.Getenv("E2E_API_BASE"); base != "" {
		return base
	}
	return "http://localhost:8080"
}

func getPodBase(podName string) string {
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return fmt.Sprintf("http://%s.scheduled-db.default.svc.cluster.local:8080", podName)
	}
	return getAPIBase()
}

func httpGet(url string, timeout time.Duration) ([]byte, int, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func httpPost(url string, payload io.Reader, timeout time.Duration) ([]byte, int, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(url, "application/json", payload)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func httpDelete(url string, timeout time.Duration) ([]byte, int, error) {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, 0, err
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func findLeaderPod(t *testing.T) string {
	t.Helper()
	pods := []string{"scheduled-db-0", "scheduled-db-1", "scheduled-db-2"}

	for _, pod := range pods {
		url := fmt.Sprintf("%s/health", getPodBase(pod))
		body, status, err := httpGet(url, 5*time.Second)
		if err != nil || status != 200 {
			continue
		}
		var health healthResponse
		if err := json.Unmarshal(body, &health); err != nil {
			continue
		}
		if health.Role == "leader" {
			return pod
		}
	}
	return ""
}

func TestClusterHasLeader(t *testing.T) {
	maxWait := 90 * time.Second
	deadline := time.Now().Add(maxWait)

	var leader string
	for time.Now().Before(deadline) {
		leader = findLeaderPod(t)
		if leader != "" {
			break
		}
		time.Sleep(3 * time.Second)
	}

	if leader == "" {
		t.Fatalf("no leader elected within %v", maxWait)
	}
	t.Logf("Leader elected: %s", leader)
}

func TestAllNodesHealthy(t *testing.T) {
	pods := []string{"scheduled-db-0", "scheduled-db-1", "scheduled-db-2"}
	maxWait := 60 * time.Second
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		allHealthy := true
		for _, pod := range pods {
			url := fmt.Sprintf("%s/health", getPodBase(pod))
			body, status, err := httpGet(url, 5*time.Second)
			if err != nil || status != 200 {
				allHealthy = false
				break
			}
			var health healthResponse
			if err := json.Unmarshal(body, &health); err != nil {
				allHealthy = false
				break
			}
			if health.Status != "ok" && health.Status != "degraded" {
				allHealthy = false
				break
			}
		}
		if allHealthy {
			t.Log("All 3 nodes are healthy")
			return
		}
		time.Sleep(3 * time.Second)
	}

	t.Fatalf("not all nodes became healthy within %v", maxWait)
}

func TestClusterConfigurationHasThreeNodes(t *testing.T) {
	leader := findLeaderPod(t)
	if leader == "" {
		t.Fatal("no leader found")
	}

	maxWait := 90 * time.Second
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		url := fmt.Sprintf("%s/debug/cluster", getPodBase(leader))
		body, status, err := httpGet(url, 5*time.Second)
		if err != nil || status != 200 {
			time.Sleep(3 * time.Second)
			continue
		}

		var debug clusterDebugResponse
		if err := json.Unmarshal(body, &debug); err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		if len(debug.Servers) >= 3 {
			t.Logf("Cluster has %d servers", len(debug.Servers))
			for _, srv := range debug.Servers {
				t.Logf("  Server: id=%s, address=%s", srv["id"], srv["address"])
			}
			return
		}
		t.Logf("Cluster has %d/3 servers, waiting...", len(debug.Servers))
		time.Sleep(3 * time.Second)
	}

	t.Fatalf("cluster did not reach 3 nodes within %v", maxWait)
}

func TestRaftReplication(t *testing.T) {
	leader := findLeaderPod(t)
	if leader == "" {
		t.Fatal("no leader found")
	}

	leaderBase := getPodBase(leader)
	jobPayload := fmt.Sprintf(`{"type":"e2e-go-test","timestamp":"%s"}`, time.Now().UTC().Format(time.RFC3339))

	body, status, err := httpPost(leaderBase+"/jobs", strings.NewReader(jobPayload), 10*time.Second)
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}
	if status != 200 {
		t.Fatalf("create job returned status %d: %s", status, string(body))
	}

	var job jobResponse
	if err := json.Unmarshal(body, &job); err != nil {
		t.Fatalf("failed to decode job response: %v", err)
	}
	t.Logf("Created job %s on leader %s", job.ID, leader)

	pods := []string{"scheduled-db-0", "scheduled-db-1", "scheduled-db-2"}
	replicationTimeout := 10 * time.Second
	deadline := time.Now().Add(replicationTimeout)

	for _, pod := range pods {
		if pod == leader {
			continue
		}
		replicated := false
		podDeadline := deadline

		for time.Now().Before(podDeadline) {
			url := fmt.Sprintf("%s/jobs/%s", getPodBase(pod), job.ID)
			rbody, rstatus, rerr := httpGet(url, 5*time.Second)
			if rerr == nil && rstatus == 200 {
				var followerJob jobResponse
				if err := json.Unmarshal(rbody, &followerJob); err == nil && followerJob.ID == job.ID {
					replicated = true
					break
				}
			}
			time.Sleep(500 * time.Millisecond)
		}

		if !replicated {
			t.Errorf("job %s not replicated to %s within %v", job.ID, pod, replicationTimeout)
		} else {
			t.Logf("Job %s replicated to %s", job.ID, pod)
		}
	}

	cleanupURL := fmt.Sprintf("%s/jobs/%s", leaderBase, job.ID)
	httpDelete(cleanupURL, 5*time.Second)
}

func TestWriteForwardingFromFollower(t *testing.T) {
	leader := findLeaderPod(t)
	if leader == "" {
		t.Fatal("no leader found")
	}

	var follower string
	pods := []string{"scheduled-db-0", "scheduled-db-1", "scheduled-db-2"}
	for _, pod := range pods {
		if pod != leader {
			follower = pod
			break
		}
	}

	followerBase := getPodBase(follower)
	jobPayload := fmt.Sprintf(`{"type":"e2e-forward-test","timestamp":"%s"}`, time.Now().UTC().Format(time.RFC3339))

	body, status, err := httpPost(followerBase+"/jobs", strings.NewReader(jobPayload), 10*time.Second)
	if err != nil {
		t.Fatalf("failed to create job via follower: %v", err)
	}
	if status != 200 {
		t.Fatalf("forward write returned status %d: %s", status, string(body))
	}

	var job jobResponse
	if err := json.Unmarshal(body, &job); err != nil {
		t.Fatalf("failed to decode forwarded job response: %v", err)
	}
	t.Logf("Write forwarding works: created job %s via follower %s", job.ID, follower)

	cleanupURL := fmt.Sprintf("%s/jobs/%s", getPodBase(leader), job.ID)
	httpDelete(cleanupURL, 5*time.Second)
}

func TestNodeJoinTiming(t *testing.T) {
	pods := []string{"scheduled-db-0", "scheduled-db-1", "scheduled-db-2"}
	maxWait := 90 * time.Second
	deadline := time.Now().Add(maxWait)

	var nodes []healthResponse
	for time.Now().Before(deadline) {
		nodes = nil
		allFound := true
		for _, pod := range pods {
			url := fmt.Sprintf("%s/health", getPodBase(pod))
			body, status, err := httpGet(url, 5*time.Second)
			if err != nil || status != 200 {
				allFound = false
				break
			}
			var health healthResponse
			if err := json.Unmarshal(body, &health); err != nil {
				allFound = false
				break
			}
			nodes = append(nodes, health)
		}
		if allFound && len(nodes) == 3 {
			break
		}
		time.Sleep(3 * time.Second)
	}

	if len(nodes) < 3 {
		t.Fatalf("only found %d/3 nodes", len(nodes))
	}

	leaderCount := 0
	for _, node := range nodes {
		t.Logf("Node: id=%s, role=%s, leader=%s", node.NodeID, node.Role, node.Leader)
		if node.Role == "leader" {
			leaderCount++
		}
	}
	if leaderCount != 1 {
		t.Errorf("expected 1 leader, got %d", leaderCount)
	}
}
