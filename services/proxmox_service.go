package services

import (
	"encoding/json"
	"fmt"
	"time"

	"cbt-core-api/proxmox"
)

type ProxmoxService interface {
	GetNodes() ([]interface{}, error)
	GetNodeStatus(node string) (map[string]interface{}, error)
	GetInstances(node string) ([]interface{}, error)
	GetInstanceRrdData(node, vmType, vmid, timeframe string) ([]interface{}, error)
	GetNodeRrdData(node, timeframe string) ([]interface{}, error)
	GetVncProxy(node, vmType, vmid string) (map[string]interface{}, error)
	VMPowerAction(node, vmType, vmid, action string) error
	UpdateVMConfig(node, vmType, vmid string, configPayload interface{}) error
	GetSnapshots(node, vmType, vmid string) ([]interface{}, error)
	CreateSnapshot(node, vmType, vmid string, payload interface{}) error
	RollbackSnapshot(node, vmType, vmid, snapname string) error
	DeleteSnapshot(node, vmType, vmid, snapname string) error
	RebuildInstance(node, vmType, vmid string) error
}

type proxmoxServiceImpl struct {
	client proxmox.ProxmoxClient
}

func NewProxmoxService(client proxmox.ProxmoxClient) ProxmoxService {
	return &proxmoxServiceImpl{client: client}
}

func (s *proxmoxServiceImpl) fetchWithCache(cacheKey string, endpoint string, ttl time.Duration) ([]byte, error) {
	if cachedData, found := proxmox.Cache.Get(cacheKey); found {
		return cachedData.([]byte), nil
	}

	body, err := s.client.Get(endpoint)
	if err != nil {
		return nil, err
	}

	proxmox.Cache.Set(cacheKey, body, ttl)
	return body, nil
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
	for _, v := range qemus {
		if m, ok := v.(map[string]interface{}); ok {
			m["type"] = "qemu"
		}
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
			m["type"] = "lxc"
		}
	}

	instances := append(qemus, lxcs...)
	proxmox.Cache.Set(cacheKey, instances, 10*time.Second)
	return instances, nil
}

func (s *proxmoxServiceImpl) GetInstanceRrdData(node, vmType, vmid, timeframe string) ([]interface{}, error) {
	cacheKey := fmt.Sprintf("rrd_%s_%s_%s_%s", node, vmType, vmid, timeframe)
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/rrddata?timeframe=%s", node, vmType, vmid, timeframe)

	body, err := s.fetchWithCache(cacheKey, endpoint, 30*time.Second)
	if err != nil {
		return nil, err
	}

	var response map[string]interface{}
	json.Unmarshal(body, &response)
	data, _ := response["data"].([]interface{})
	return data, nil
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

func (s *proxmoxServiceImpl) GetVncProxy(node, vmType, vmid string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/vncproxy", node, vmType, vmid)
	
	// Create x509 parameter for proxy
	payload := map[string]interface{}{
		"websocket": 1,
	}

	body, err := s.client.Post(endpoint, payload)
	if err != nil {
		return nil, err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	data, _ := response["data"].(map[string]interface{})
	return data, nil
}

func (s *proxmoxServiceImpl) VMPowerAction(node, vmType, vmid, action string) error {
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/status/%s", node, vmType, vmid, action)
	_, err := s.client.Post(endpoint, nil)
	if err == nil {
		proxmox.Cache.Delete(fmt.Sprintf("instances_%s", node))
	}
	return err
}

func (s *proxmoxServiceImpl) UpdateVMConfig(node, vmType, vmid string, configPayload interface{}) error {
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/config", node, vmType, vmid)
	_, err := s.client.Post(endpoint, configPayload)
	return err
}

func (s *proxmoxServiceImpl) GetSnapshots(node, vmType, vmid string) ([]interface{}, error) {
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/snapshot", node, vmType, vmid)
	body, err := s.client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	var response map[string]interface{}
	json.Unmarshal(body, &response)
	data, _ := response["data"].([]interface{})
	return data, nil
}

func (s *proxmoxServiceImpl) CreateSnapshot(node, vmType, vmid string, payload interface{}) error {
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/snapshot", node, vmType, vmid)
	_, err := s.client.Post(endpoint, payload)
	return err
}

func (s *proxmoxServiceImpl) RollbackSnapshot(node, vmType, vmid, snapname string) error {
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/snapshot/%s/rollback", node, vmType, vmid, snapname)
	_, err := s.client.Post(endpoint, nil)
	if err == nil {
		proxmox.Cache.Delete(fmt.Sprintf("nodestatus_%s", node))
		proxmox.Cache.Delete(fmt.Sprintf("instances_%s", node))
		proxmox.Cache.Delete(fmt.Sprintf("%s_%s", vmType, node))
	}
	return err
}

func (s *proxmoxServiceImpl) DeleteSnapshot(node, vmType, vmid, snapname string) error {
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/snapshot/%s", node, vmType, vmid, snapname)
	_, err := s.client.Delete(endpoint)
	return err
}

func (s *proxmoxServiceImpl) RebuildInstance(node, vmType, vmid string) error {
	return nil // Mocked for now
}
