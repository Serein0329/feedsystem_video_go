package mysql

import (
	"errors"
	"feedsystem_video_go/internal/account"
	"feedsystem_video_go/internal/config"
	"feedsystem_video_go/internal/message"
	"feedsystem_video_go/internal/social"
	"feedsystem_video_go/internal/sse"
	"feedsystem_video_go/internal/video"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func NewDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	if cfg == nil {
		return nil, errors.New("database config is nil")
	}
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		TranslateError:                           true,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&account.Account{}, &video.Video{}, &video.Like{}, &video.Comment{},
		&social.Social{}, &video.Tag{}, &video.VideoTag{},
		&message.Message{}, &sse.Notification{},
	)
}

func CloseDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
