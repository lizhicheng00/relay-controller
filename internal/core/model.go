package core

type Tunnel struct {
	ID              uint64
	Name            string
	TunnelID        string
	TunnelCode      uint64
	ClusterID       string
	Expiration      int64
	ExpirationHours int
	Namespace       string
	AccountID       uint64
	Description     *string
	BandwidthUsed   uint64
	URL             string
	Type            string
	CreatedAt       int64
	UpdatedAt       int64
	PortCount       uint64
}

type TunnelPort struct {
	ID             uint64
	TunnelCode     uint64
	Port           uint16
	Protocol       string
	AllowAnonymous bool
}

type TunnelStatus struct {
	HostConnectionCount    uint32 `json:"hostConnectionCount"`
	ClientConnectionCount  uint32 `json:"clientConnectionCount"`
	UploadBytesPerSecond   uint64 `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond uint64 `json:"downloadBytesPerSecond"`
	TotalUploadBytes       uint64 `json:"totalUploadBytes"`
	TotalDownloadBytes     uint64 `json:"totalDownloadBytes"`
	ReportedAt             int64  `json:"reportedAt"`
}

type CreateTunnelRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	ClusterID   string  `json:"clusterId"`
	Expiration  *int    `json:"expiration"`
	Type        *string `json:"type"`
}

type UpdateTunnelRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Expiration  *int    `json:"expiration"`
	Type        *string `json:"type"`
}

type CreateTunnelPortRequest struct {
	Port           *int64  `json:"port"`
	Protocol       *string `json:"protocol"`
	AllowAnonymous *bool   `json:"allowAnonymous"`
}

type UpdateTunnelPortRequest struct {
	Protocol       *string `json:"protocol"`
	AllowAnonymous *bool   `json:"allowAnonymous"`
}

type TunnelResponse struct {
	Name             string        `json:"name"`
	TunnelID         string        `json:"tunnelId"`
	TunnelCode       uint64        `json:"tunnelCode"`
	ClusterID        string        `json:"clusterId"`
	Description      *string       `json:"description"`
	BandwidthUsed    uint64        `json:"bandwidthUsed"`
	ExpirationHours  int           `json:"expirationHours"`
	TunnelExpiration int64         `json:"tunnelExpiration"`
	Created          int64         `json:"created"`
	URL              string        `json:"url"`
	Type             string        `json:"type"`
	Status           *TunnelStatus `json:"status,omitempty"`
}

type TunnelListItem struct {
	TunnelID         string  `json:"tunnelId"`
	TunnelCode       uint64  `json:"tunnelCode"`
	ClusterID        string  `json:"clusterId"`
	Name             string  `json:"name"`
	Description      *string `json:"description"`
	ExpirationHours  int     `json:"expirationHours"`
	TunnelExpiration int64   `json:"tunnelExpiration"`
	Created          int64   `json:"created"`
	URL              string  `json:"url"`
	PortCount        uint64  `json:"portCount"`
}

type TunnelPortResponse struct {
	TunnelID       string `json:"tunnelId"`
	TunnelCode     uint64 `json:"tunnelCode"`
	Port           uint16 `json:"port"`
	Protocol       string `json:"protocol"`
	AllowAnonymous bool   `json:"allowAnonymous"`
}

type TunnelTokenResponse struct {
	TunnelID   string `json:"tunnelId"`
	Scope      string `json:"scope"`
	Lifetime   int64  `json:"lifetime"`
	Expiration int64  `json:"expiration"`
	Token      string `json:"token"`
}

type LimitsResponse struct {
	ResetAt                          int64  `json:"resetAt"`
	QuotaBytes                       uint64 `json:"quotaBytes"`
	RemainingBytes                   uint64 `json:"remainingBytes"`
	ActiveTunnels                    uint64 `json:"activeTunnels"`
	MaxTunnels                       int    `json:"maxTunnels"`
	MaxPortsPerTunnel                int    `json:"maxPortsPerTunnel"`
	MaxHostsPerTunnel                int    `json:"maxHostsPerTunnel"`
	MaxTunnelBandwidthBytesPerSecond uint64 `json:"maxTunnelBandwidthBytesPerSecond"`
	MaxHTTPRequestsPerMinutePerPort  int    `json:"maxHttpRequestsPerMinutePerPort"`
	MaxConnectionsPerPort            int    `json:"maxConnectionsPerPort"`
}

type BillingAccount struct {
	ID       uint64
	PlanCode string
	Status   string
}

type BillingPlan struct {
	MonthlyQuotaBytes                uint64
	MaxTunnels                       int
	MaxPortsPerTunnel                int
	MaxHostsPerTunnel                int
	MaxTunnelBandwidthBytesPerSecond uint64
	MaxHTTPRequestsPerMinutePerPort  int
	MaxConnectionsPerPort            int
}

type BillingPeriod struct {
	End         int64
	QuotaBytes  uint64
	BilledBytes uint64
}

type MeteringRecord struct {
	ID            uint64
	AccountID     uint64
	TunnelID      string
	UploadBytes   uint64
	DownloadBytes uint64
	ReportedAt    int64
}

func (t Tunnel) Response(status *TunnelStatus) TunnelResponse {
	return TunnelResponse{
		Name: t.Name, TunnelID: t.TunnelID, TunnelCode: t.TunnelCode, ClusterID: t.ClusterID,
		Description: t.Description, BandwidthUsed: t.BandwidthUsed, ExpirationHours: t.ExpirationHours,
		TunnelExpiration: t.Expiration, Created: t.CreatedAt, URL: t.URL, Type: t.Type, Status: status,
	}
}

func (t Tunnel) ListItem() TunnelListItem {
	return TunnelListItem{
		TunnelID: t.TunnelID, TunnelCode: t.TunnelCode, ClusterID: t.ClusterID, Name: t.Name,
		Description: t.Description, ExpirationHours: t.ExpirationHours, TunnelExpiration: t.Expiration,
		Created: t.CreatedAt, URL: t.URL, PortCount: t.PortCount,
	}
}

func PortResponse(tunnel Tunnel, port TunnelPort) TunnelPortResponse {
	return TunnelPortResponse{
		TunnelID: tunnel.TunnelID, TunnelCode: port.TunnelCode, Port: port.Port,
		Protocol: port.Protocol, AllowAnonymous: port.AllowAnonymous,
	}
}
