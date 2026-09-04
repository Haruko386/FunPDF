package entity

// RuntimeInfo keeps a single persisted runtime record (desktop mode, ID always 1)
// so the effective cache dir can survive restarts.
type RuntimeInfo struct {
	ID           uint   `gorm:"primary_key" json:"id"`
	Mode         string `json:"mode"`
	Version      string `json:"version"`
	Database     string `json:"database"`
	DatabasePath string `json:"database_path,omitempty"`
	CacheDir     string `json:"cache_dir"`
	BaseModel
}
