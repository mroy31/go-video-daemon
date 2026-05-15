package library

import (
	"time"
)

type Library struct {
	ID   uint `gorm:"primaryKey"`
	Name string
	Path string

	LibraryFolders []LibraryFolder
}

type LibraryFolder struct {
	ID   uint `gorm:"primaryKey"`
	Name string
	Path string

	LibraryID uint
	ParentID  *uint
	Library   Library `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Parent    *LibraryFolder

	Childs []LibraryFolder `gorm:"foreignKey:ParentID"`
	Videos []Video
}

type Video struct {
	ID           uint `gorm:"primaryKey"`
	Name         string
	Path         string
	Duration     float64
	LastPosition int
	PlayedAt     time.Time

	LibraryFolderID uint
	LibraryFolder   LibraryFolder `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	AudioStreams    []AudioStream
	SubtitleStreams []SubtitleStream
}

type AudioStream struct {
	ID   uint `gorm:"primaryKey"`
	Lang string
	Idx  int

	VideoID uint
	Video   Video `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type SubtitleStream struct {
	ID   uint `gorm:"primaryKey"`
	Lang string
	Idx  int

	VideoID uint
	Video   Video `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
