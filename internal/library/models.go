package library

import (
	"time"

	"gorm.io/gorm"
)

type Library struct {
	gorm.Model
	Name string
	Path string

	LibraryFolders []LibraryFolder
}

type LibraryFolder struct {
	gorm.Model
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
	gorm.Model
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
	gorm.Model
	Lang string
	Idx  int

	VideoID uint
	Video   Video `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type SubtitleStream struct {
	gorm.Model
	Lang string
	Idx  int

	VideoID uint
	Video   Video `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
