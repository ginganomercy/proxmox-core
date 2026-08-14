package services

import (
	"time"

	"cbt-core-api/pkg/proxmox"
)

// ProxmoxService defines the contract for interacting with the underlying Proxmox VE REST API.
// It abstracts away raw HTTP calls and provides domain-specific methods for cluster and VM management.
type ProxmoxService interface {
	GetNodes() ([]interface{}, error)
	GetNodeStatus(node string) (map[string]interface{}, error)
	GetInstances(node string) ([]interface{}, error)
	GetInstanceIP(node, vmType, vmid string) (string, error)
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
	CloneVM(node, baseVmid, newVmid string, name string) (string, error)
	ResizeDisk(node, vmType, vmid, disk, size string) error
	// Production-grade: Get next available VMID from cluster
	GetNextVMID() (string, error)
	// Production-grade: Poll a Proxmox task until completion or timeout
	WaitForTask(node, upid string) error
	// Production-grade: Delete a VM for rollback purposes
	DeleteVM(node, vmid string) error
	GetClusterLogs() ([]interface{}, error)
	GetClusterTasks() ([]interface{}, error)
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
