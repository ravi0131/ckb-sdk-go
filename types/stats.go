package types

import "math/big"

type AlertMessage struct {
	Id          uint32 `json:"id"`
	Message     string `json:"message"`
	NoticeUntil uint64 `json:"notice_until"`
	Priority    uint32 `json:"priority"`
}

type BlockchainInfo struct {
	Alerts                 []*AlertMessage `json:"alerts"`
	Chain                  string          `json:"chain"`
	Difficulty             *big.Int        `json:"difficulty"`
	Epoch                  uint64          `json:"epoch"`
	IsInitialBlockDownload bool            `json:"is_initial_block_download"`
	MedianTime             uint64          `json:"median_time"`
}

// DeploymentState represents the possible states of a deployment.
type DeploymentState int

const (
	// Defined is the first state that each softfork starts.
	Defined DeploymentState = iota
	// Started is the state for epochs past the `start` epoch.
	Started
	// LockedIn is the state for epochs after the first epoch period with STARTED epochs of which at least `threshold` has the associated bit set in `version`.
	LockedIn
	// Active is the state for all epochs after the LOCKED_IN epoch.
	Active
	// Failed is the state for epochs past the `timeout_epoch`, if LOCKED_IN was not reached.
	Failed
)

// DeploymentPos represents the possible positions for deployments.
type DeploymentPos int

const (
	// Testdummy represents a dummy deployment.
	Testdummy DeploymentPos = iota
	// LightClient represents the light client protocol deployment.
	LightClient
)

// DeploymentInfo represents information about a deployment.
type DeploymentInfo struct {
	Bit                uint8
	Start              uint64
	Timeout            uint64
	MinActivationEpoch uint64
	Period             uint64
	Threshold          RationalU256
	Since              uint64
	State              DeploymentState
}

// DeploymentsInfo represents information about multiple deployments.
type DeploymentsInfo struct {
	Hash        Hash
	Epoch       uint64
	Deployments map[DeploymentPos]DeploymentInfo
}
