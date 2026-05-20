// Package cli builds the authnet command tree and local command behavior.
package cli

const (
	defaultVersion        = "0.0.0-dev"
	defaultSchemaVersion  = "0.2.0"
	defaultContractStatus = "alpha"
)

// BuildInfo carries local build metadata. It is intentionally local-only:
// version reporting must never perform update checks or network requests.
type BuildInfo struct {
	Version        string
	Commit         string
	Date           string
	SchemaVersion  string
	ContractStatus string
}

func (info BuildInfo) normalized() BuildInfo {
	if info.Version == "" {
		info.Version = defaultVersion
	}
	if info.Commit == "" {
		info.Commit = "unknown"
	}
	if info.Date == "" {
		info.Date = "unknown"
	}
	if info.SchemaVersion == "" {
		info.SchemaVersion = defaultSchemaVersion
	}
	if info.ContractStatus == "" {
		info.ContractStatus = defaultContractStatus
	}
	return info
}
