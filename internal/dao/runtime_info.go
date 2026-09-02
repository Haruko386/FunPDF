package dao

import (
	"FunPDF/internal/entity"
	"context"

	"gorm.io/gorm"
)

const runtimeInfoID = 1

type RuntimeInfoDAO struct{}

func NewRuntimeInfoDAO() *RuntimeInfoDAO {
	return &RuntimeInfoDAO{}
}

// Get returns the single runtime info row. When no row exists, yet it returns a
// zero value with a nil error, so callers should check info.ID == 0.
func (d *RuntimeInfoDAO) Get(ctx context.Context, db *gorm.DB) (entity.RuntimeInfo, error) {
	var info entity.RuntimeInfo
	err := db.WithContext(ctx).Where("id = ?", runtimeInfoID).Limit(1).Find(&info).Error
	return info, err
}

// Save upserts the single runtime info row.
func (d *RuntimeInfoDAO) Save(ctx context.Context, db *gorm.DB, info *entity.RuntimeInfo) error {
	info.ID = runtimeInfoID
	return db.WithContext(ctx).Save(info).Error
}

// UpdateCacheDir updates cache_dir of the single row, creating it when missing.
func (d *RuntimeInfoDAO) UpdateCacheDir(ctx context.Context, db *gorm.DB, dir string) error {
	info, err := d.Get(ctx, db)
	if err != nil {
		return err
	}
	if info.ID == 0 {
		info = entity.RuntimeInfo{ID: runtimeInfoID, CacheDir: dir}
		return db.WithContext(ctx).Save(&info).Error
	}
	info.CacheDir = dir
	return db.WithContext(ctx).Save(&info).Error
}
