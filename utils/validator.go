package utils

import (
	"regexp"
)

var (
	// nodeRegex matches valid Proxmox node names (alphanumeric and hyphens, up to 64 chars)
	nodeRegex = regexp.MustCompile(`^[a-zA-Z0-9-]{1,64}$`)

	// vmidRegex matches valid VM IDs (only digits, up to 10 chars)
	vmidRegex = regexp.MustCompile(`^\d{1,10}$`)

	// typeRegex matches strictly "qemu" or "lxc"
	typeRegex = regexp.MustCompile(`^(qemu|lxc)$`)
)

// IsValidNode checks if the given string is a valid node name to prevent Path Traversal
func IsValidNode(node string) bool {
	return nodeRegex.MatchString(node)
}

// IsValidVMID checks if the given string is a valid VM ID
func IsValidVMID(vmid string) bool {
	return vmidRegex.MatchString(vmid)
}

// IsValidVMType checks if the given string is either "qemu" or "lxc"
func IsValidVMType(t string) bool {
	return typeRegex.MatchString(t)
}
