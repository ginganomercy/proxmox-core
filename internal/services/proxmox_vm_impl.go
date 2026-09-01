package services

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"time"

	"cbt-core-api/pkg/proxmox"
)

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

func (s *proxmoxServiceImpl) GetTermProxy(node, vmType, vmid string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/termproxy", node, vmType, vmid)

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

// GetNextVMID calls the Proxmox cluster API to get the next available unique VMID.
// This guarantees no collision between concurrent VM provisioning requests.
func (s *proxmoxServiceImpl) GetNextVMID() (string, error) {
	body, err := s.client.Get("/cluster/nextid")
	if err != nil {
		return "", fmt.Errorf("failed to get next VMID from cluster: %w", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse next VMID response: %w", err)
	}

	// Proxmox usually returns the VMID as an integer/float64, not a string
	switch v := resp["data"].(type) {
	case string:
		if v == "" {
			return "", fmt.Errorf("invalid VMID returned from cluster")
		}
		return v, nil
	case float64:
		return fmt.Sprintf("%.0f", v), nil
	default:
		return "", fmt.Errorf("invalid VMID format returned from cluster")
	}
}

// WaitForTask polls a Proxmox UPID task until it completes, fails, or times out.
//
// IMPORTANT: Proxmox API task status schema:
//   - data.status == "running"  → task still in progress, keep polling
//   - data.status == "stopped"  → task finished; check data.exitstatus
//     - data.exitstatus == "OK"      → success
//     - data.exitstatus == anything else → failure with that message
//
// Common mistake: checking status=="OK" directly — Proxmox NEVER returns that.
func (s *proxmoxServiceImpl) WaitForTask(node, upid string) error {
	// URL-encode the UPID: it contains special chars like ':', '|', '@'
	// which must be percent-encoded in URL paths to avoid API routing errors.
	encodedUpid := url.PathEscape(upid)
	endpoint := fmt.Sprintf("/nodes/%s/tasks/%s/status", node, encodedUpid)

	const POLL_INTERVAL = 3 * time.Second
	const TIMEOUT = 10 * time.Minute

	deadline := time.Now().Add(TIMEOUT)

	for time.Now().Before(deadline) {
		body, err := s.client.Get(endpoint)
		if err != nil {
			// Transient error (network, 5xx) — retry on next poll cycle
			log.Printf("[WARN] WaitForTask poll error for %s: %v (retrying)", upid, err)
			time.Sleep(POLL_INTERVAL)
			continue
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse task status response: %w", err)
		}

		data, ok := resp["data"].(map[string]interface{})
		if !ok {
			time.Sleep(POLL_INTERVAL)
			continue
		}

		// Proxmox task lifecycle: running → stopped
		status, _ := data["status"].(string)

		switch status {
		case "stopped":
			// Task has finished — inspect exitstatus for success/failure
			exitStatus, _ := data["exitstatus"].(string)
			if exitStatus == "OK" {
				return nil // ✅ Success
			}
			// Any non-OK exitstatus means failure (e.g. "error", "interrupted")
			return fmt.Errorf("proxmox task failed with exitstatus: %q", exitStatus)
		case "running":
			// Still running — keep polling
		default:
			// Unknown status — log and keep polling defensively
			log.Printf("[WARN] WaitForTask: unexpected status %q for task %s", status, upid)
		}

		time.Sleep(POLL_INTERVAL)
	}

	return fmt.Errorf("task %s timed out after %v", upid, TIMEOUT)
}

// DeleteVM deletes a QEMU VM from a Proxmox node. Used for rollback on provisioning failure.
func (s *proxmoxServiceImpl) DeleteVM(node, vmid string) error {
	// Force stop first, ignore error if already stopped
	_ = s.VMPowerAction(node, "qemu", vmid, "stop")
	time.Sleep(2 * time.Second)

	endpoint := fmt.Sprintf("/nodes/%s/qemu/%s", node, vmid)
	_, err := s.client.Delete(endpoint)
	if err == nil {
		proxmox.Cache.Delete(fmt.Sprintf("instances_%s", node))
	}
	return err
}

// CloneVM clones a VM template and returns the Proxmox UPID task ID.
// The caller MUST call WaitForTask with the returned UPID before modifying the new VM.
func (s *proxmoxServiceImpl) CloneVM(node, baseVmid, newVmid string, name string) (string, error) {
	endpoint := fmt.Sprintf("/nodes/%s/qemu/%s/clone", node, baseVmid)
	payload := map[string]interface{}{
		"newid": newVmid,
		"name":  name,
		"full":  1, // Full clone — independent of template storage
	}
	body, err := s.client.Post(endpoint, payload)
	if err != nil {
		return "", err
	}

	// Proxmox returns the UPID task ID in the 'data' field
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse clone response: %w", err)
	}

	upid, ok := resp["data"].(string)
	if !ok || upid == "" {
		return "", fmt.Errorf("no UPID returned from clone operation")
	}

	proxmox.Cache.Delete(fmt.Sprintf("instances_%s", node))
	return upid, nil
}

func (s *proxmoxServiceImpl) ResizeDisk(node, vmType, vmid, disk, size string) error {
	endpoint := fmt.Sprintf("/nodes/%s/%s/%s/resize", node, vmType, vmid)
	payload := map[string]interface{}{
		"disk": disk,
		"size": size,
	}
	_, err := s.client.Put(endpoint, payload)
	return err
}

func (s *proxmoxServiceImpl) RebuildInstance(node, vmType, vmid string) error {
	if vmType != "qemu" {
		return fmt.Errorf("rebuild only supported for qemu")
	}

	// 1. Force Stop VM
	s.VMPowerAction(node, vmType, vmid, "stop")
	time.Sleep(3 * time.Second)

	// 2. Delete VM
	endpointDel := fmt.Sprintf("/nodes/%s/%s/%s", node, vmType, vmid)
	_, err := s.client.Delete(endpointDel)
	if err != nil {
		return fmt.Errorf("failed to delete existing VM: %v", err)
	}

	// Wait for deletion
	time.Sleep(5 * time.Second)

	// 3. Clone from Golden Image and wait for task to complete
	upid, err := s.CloneVM(node, "100", vmid, "Rebuilt-VM-"+vmid)
	if err != nil {
		return fmt.Errorf("failed to clone from golden image: %v", err)
	}

	// Wait for clone task to fully complete before proceeding
	if err := s.WaitForTask(node, upid); err != nil {
		return fmt.Errorf("clone task failed during rebuild: %v", err)
	}

	proxmox.Cache.Delete(fmt.Sprintf("instances_%s", node))
	return nil
}

func (s *proxmoxServiceImpl) GetInstanceIP(node, vmType, vmid string) (string, error) {
	// For QEMU, try to get from QEMU guest agent
	if vmType == "qemu" {
		endpoint := fmt.Sprintf("/nodes/%s/qemu/%s/agent/network-get-interfaces", node, vmid)
		body, err := s.client.Get(endpoint)
		if err == nil {
			var resp map[string]interface{}
			if err := json.Unmarshal(body, &resp); err == nil {
				if result, ok := resp["data"].(map[string]interface{})["result"].([]interface{}); ok {
					for _, intfRaw := range result {
						if intf, ok := intfRaw.(map[string]interface{}); ok {
							if name, _ := intf["name"].(string); name == "lo" {
								continue
							}
							if ipAddresses, ok := intf["ip-addresses"].([]interface{}); ok {
								for _, ipRaw := range ipAddresses {
									if ipObj, ok := ipRaw.(map[string]interface{}); ok {
										if ipType, _ := ipObj["ip-address-type"].(string); ipType == "ipv4" {
											if ip, _ := ipObj["ip-address"].(string); ip != "" && ip != "127.0.0.1" {
												return ip, nil
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	} else if vmType == "lxc" {
		// For LXC, try to get from interfaces
		endpoint := fmt.Sprintf("/nodes/%s/lxc/%s/interfaces", node, vmid)
		body, err := s.client.Get(endpoint)
		if err == nil {
			var resp map[string]interface{}
			if err := json.Unmarshal(body, &resp); err == nil {
				if data, ok := resp["data"].([]interface{}); ok {
					for _, intfRaw := range data {
						if intf, ok := intfRaw.(map[string]interface{}); ok {
							if name, _ := intf["name"].(string); name == "lo" {
								continue
							}
							if inet, _ := intf["inet"].(string); inet != "" && inet != "127.0.0.1/8" {
								// Format is usually IP/CIDR (e.g. 192.168.1.100/24)
								// Strip the CIDR part if needed, but returning it is fine or we can parse it.
								return inet, nil
							}
						}
					}
				}
			}
		}
	}
	return "", fmt.Errorf("IP address not found")
}
