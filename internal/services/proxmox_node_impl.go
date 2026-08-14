package services

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"cbt-core-api/pkg/proxmox"
)

func (s *proxmoxServiceImpl) GetClusterLogs() ([]interface{}, error) {
	cacheKey := "cluster_logs"
	body, err := s.fetchWithCache(cacheKey, "/cluster/log?max=500", 5*time.Second)
	if err != nil {
		return nil, err
	}

	var response map[string]interface{}
	json.Unmarshal(body, &response)
	data, _ := response["data"].([]interface{})
	return data, nil
}

func (s *proxmoxServiceImpl) GetClusterTasks() ([]interface{}, error) {
	nodes, err := s.GetNodes()
	if err != nil || len(nodes) == 0 {
		return nil, fmt.Errorf("failed to retrieve cluster nodes for tasks")
	}

	var allTasks []interface{}
	for _, nodeObj := range nodes {
		if nodeMap, ok := nodeObj.(map[string]interface{}); ok {
			if nodeName, ok := nodeMap["node"].(string); ok {
				cacheKey := fmt.Sprintf("node_tasks_%s", nodeName)
				body, err := s.fetchWithCache(cacheKey, fmt.Sprintf("/nodes/%s/tasks", nodeName), 5*time.Second)
				if err == nil {
					var resp map[string]interface{}
					if json.Unmarshal(body, &resp) == nil {
						if tasks, ok := resp["data"].([]interface{}); ok {
							for _, taskObj := range tasks {
								if taskMap, ok := taskObj.(map[string]interface{}); ok {
									taskMap["node"] = nodeName
									allTasks = append(allTasks, taskMap)
								}
							}
						}
					}
				}
			}
		}
	}

	sort.Slice(allTasks, func(i, j int) bool {
		m1, ok1 := allTasks[i].(map[string]interface{})
		m2, ok2 := allTasks[j].(map[string]interface{})
		if !ok1 || !ok2 {
			return false
		}
		t1, _ := m1["starttime"].(float64)
		t2, _ := m2["starttime"].(float64)
		return t1 > t2
	})

	return allTasks, nil
}

func (s *proxmoxServiceImpl) GetNodes() ([]interface{}, error) {
	cacheKey := "nodes_list"
	body, err := s.fetchWithCache(cacheKey, "/nodes", 1*time.Minute)
	if err != nil {
		return nil, err
	}

	var response map[string]interface{}
	json.Unmarshal(body, &response)
	data, _ := response["data"].([]interface{})
	return data, nil
}

func (s *proxmoxServiceImpl) GetNodeStatus(node string) (map[string]interface{}, error) {
	cacheKey := fmt.Sprintf("nodestatus_%s", node)
	body, err := s.fetchWithCache(cacheKey, fmt.Sprintf("/nodes/%s/status", node), 10*time.Second)
	if err != nil {
		return nil, err
	}

	var response map[string]interface{}
	json.Unmarshal(body, &response)
	data, _ := response["data"].(map[string]interface{})

	// Fetch storage pool info for real node capacity
	storageCacheKey := fmt.Sprintf("nodestorage_%s", node)
	storageBody, err := s.fetchWithCache(storageCacheKey, fmt.Sprintf("/nodes/%s/storage", node), 10*time.Second)
	if err == nil {
		var storageResp map[string]interface{}
		if json.Unmarshal(storageBody, &storageResp) == nil {
			if storages, ok := storageResp["data"].([]interface{}); ok {
				var totalStorage float64 = 0
				var usedStorage float64 = 0
				for _, storeRaw := range storages {
					if store, ok := storeRaw.(map[string]interface{}); ok {
						if active, ok := store["active"].(float64); ok && active == 1 {
							if t, ok := store["total"].(float64); ok {
								totalStorage += t
							}
							if u, ok := store["used"].(float64); ok {
								usedStorage += u
							}
						}
					}
				}
				data["storage_total"] = totalStorage
				data["storage_used"] = usedStorage
			}
		}
	}

	return data, nil
}

func (s *proxmoxServiceImpl) GetInstances(node string) ([]interface{}, error) {
	cacheKey := fmt.Sprintf("instances_%s", node)

	if cachedData, found := proxmox.Cache.Get(cacheKey); found {
		return cachedData.([]interface{}), nil
	}

	qemuBody, err := s.fetchWithCache(fmt.Sprintf("qemu_%s", node), fmt.Sprintf("/nodes/%s/qemu", node), 10*time.Second)
	if err != nil {
		return nil, err
	}
	var qemuResp map[string]interface{}
	json.Unmarshal(qemuBody, &qemuResp)
	qemus, _ := qemuResp["data"].([]interface{})
	var instances []interface{}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, v := range qemus {
		if m, ok := v.(map[string]interface{}); ok {
			// Skip templates
			if template, ok := m["template"]; ok && (template == float64(1) || template == 1) {
				continue
			}
			m["type"] = "qemu"
			
			// Inject into instances list first (so pointers/references are established)
			instances = append(instances, m)

			// If VM is running, fetch guest-agent fsinfo concurrently
			status, _ := m["status"].(string)
			if status == "running" {
				if vmid, ok := m["vmid"].(float64); ok {
					wg.Add(1)
					go func(vmid float64, vmMap map[string]interface{}) {
						defer wg.Done()
						agentEndpoint := fmt.Sprintf("/nodes/%s/qemu/%.0f/agent/get-fsinfo", node, vmid)
						agentBody, err := s.client.Get(agentEndpoint)
						if err == nil {
							var agentResp map[string]interface{}
							if json.Unmarshal(agentBody, &agentResp) == nil {
								if result, ok := agentResp["data"].(map[string]interface{})["result"].([]interface{}); ok {
									var totalUsedBytes float64 = 0
									for _, fsRaw := range result {
										if fs, ok := fsRaw.(map[string]interface{}); ok {
											if used, ok := fs["used-bytes"].(float64); ok {
												totalUsedBytes += used
											}
										}
									}
									if totalUsedBytes > 0 {
										mu.Lock()
										vmMap["disk"] = totalUsedBytes
										mu.Unlock()
									}
								}
							}
						}
					}(vmid, m)
				}
			}
		}
	}

	// Wait for all QEMU Guest Agent polls with 3s timeout to prevent API hangs
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()
	select {
	case <-done:
		// All done
	case <-time.After(3 * time.Second):
		log.Printf("[WARN] QEMU Guest Agent fsinfo polling timed out on node %s", node)
	}

	lxcBody, err := s.fetchWithCache(fmt.Sprintf("lxc_%s", node), fmt.Sprintf("/nodes/%s/lxc", node), 10*time.Second)
	if err != nil {
		return nil, err
	}
	var lxcResp map[string]interface{}
	json.Unmarshal(lxcBody, &lxcResp)
	lxcs, _ := lxcResp["data"].([]interface{})

	for _, v := range lxcs {
		if m, ok := v.(map[string]interface{}); ok {
			// Skip templates
			if template, ok := m["template"]; ok && (template == float64(1) || template == 1) {
				continue
			}
			m["type"] = "lxc"
			instances = append(instances, m)
		}
	}

	proxmox.Cache.Set(cacheKey, instances, 10*time.Second)
	return instances, nil
}

func (s *proxmoxServiceImpl) GetNodeRrdData(node, timeframe string) ([]interface{}, error) {
	cacheKey := fmt.Sprintf("noderrd_%s_%s", node, timeframe)
	endpoint := fmt.Sprintf("/nodes/%s/rrddata?timeframe=%s", node, timeframe)

	body, err := s.fetchWithCache(cacheKey, endpoint, 30*time.Second)
	if err != nil {
		return nil, err
	}

	var response map[string]interface{}
	json.Unmarshal(body, &response)
	data, _ := response["data"].([]interface{})
	return data, nil
}
