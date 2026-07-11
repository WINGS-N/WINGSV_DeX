package config

// The auto-search results are tagged into a synthetic subscription so the Profiles screen
// shows them under an "Автопоиск" filter chip alongside the real subscriptions.
const (
	AutoSearchSubscriptionID    = "__autosearch__"
	AutoSearchSubscriptionTitle = "Автопоиск"
)

// AutoSearchSettings are the probe parameters shown before a run.
type AutoSearchSettings struct {
	TargetCount            int  `json:"targetCount"`            // how many stable profiles to find
	TCPingTimeoutMs        int  `json:"tcpingTimeoutMs"`        // per-profile TCP connect timeout
	DownloadSizeMb         int  `json:"downloadSizeMb"`         // test-file size
	DownloadTimeoutSeconds int  `json:"downloadTimeoutSeconds"` // per-attempt download timeout
	DownloadAttempts       int  `json:"downloadAttempts"`       // runs that all must pass to be "stable"
	UseBuiltInSubscription bool `json:"useBuiltInSubscription"` // seed + probe the Universal list
}

// DefaultAutoSearchSettings mirrors the app's defaults.
func DefaultAutoSearchSettings() AutoSearchSettings {
	return AutoSearchSettings{
		TargetCount:            5,
		TCPingTimeoutMs:        1000,
		DownloadSizeMb:         5,
		DownloadTimeoutSeconds: 20,
		DownloadAttempts:       2,
		UseBuiltInSubscription: true,
	}
}

func clampInt(v, lo, hi, def int) int {
	if v == 0 {
		return def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// normalized clamps each field to the app's accepted range.
func (a AutoSearchSettings) normalized() AutoSearchSettings {
	d := DefaultAutoSearchSettings()
	a.TargetCount = clampInt(a.TargetCount, 1, 20, d.TargetCount)
	a.TCPingTimeoutMs = clampInt(a.TCPingTimeoutMs, 300, 10000, d.TCPingTimeoutMs)
	a.DownloadSizeMb = clampInt(a.DownloadSizeMb, 1, 100, d.DownloadSizeMb)
	a.DownloadTimeoutSeconds = clampInt(a.DownloadTimeoutSeconds, 3, 120, d.DownloadTimeoutSeconds)
	a.DownloadAttempts = clampInt(a.DownloadAttempts, 1, 10, d.DownloadAttempts)
	return a
}
