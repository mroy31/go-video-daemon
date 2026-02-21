package library

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mroy31/go-video-daemon/internal/config"
	"github.com/sirupsen/logrus"
)

type VideoLibrary struct {
	Db         *gorm.DB
	Library    *Library
	rootFolder *LibraryFolder
	logger     *logrus.Entry
}

func (v *VideoLibrary) createRootFolder() error {
	v.rootFolder = &LibraryFolder{
		Name:      "",
		Path:      v.Library.Path,
		ParentID:  nil,
		LibraryID: v.Library.ID,
	}

	result := v.Db.Create(v.rootFolder)
	return result.Error
}

func (v *VideoLibrary) Init(name string, path string) error {
	result := v.Db.First(&v.Library, Library{Name: name})
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		v.logger.Infof("Create model in the database")

		v.Library = &Library{Name: name, Path: path}
		result := v.Db.Create(v.Library)
		if result.Error != nil {
			return fmt.Errorf("unable to create library model: %v", result.Error)
		}

		return v.createRootFolder()
	}

	if v.Library.Path != path { // entry already exists and root folder changes
		// remove all folder and videos since root folder  has been updated
		ctx := context.Background()
		_, err := gorm.G[LibraryFolder](v.Db).Where("library_id = ?", v.Library.ID).Delete(ctx)
		if err != nil {
			return err
		}

		// save new path
		v.Library.Path = path
		if r := v.Db.Save(&v.Library); r.Error != nil {
			return r.Error
		}
		return v.createRootFolder()
	}

	return v.Db.First(&v.rootFolder, LibraryFolder{Name: "", LibraryID: v.Library.ID}).Error
}

func (v *VideoLibrary) createLibraryFolder(parentID *uint, path string, name string) (*LibraryFolder, error) {
	libraryFolder := &LibraryFolder{
		Name:      name,
		Path:      path,
		ParentID:  parentID,
		LibraryID: v.Library.ID,
	}

	result := v.Db.Create(libraryFolder)
	return libraryFolder, result.Error
}

func (v *VideoLibrary) getLibraryFolder(recordedFolders []LibraryFolder, path string) *LibraryFolder {
	for _, f := range recordedFolders {
		if f.Path == path {
			return &f
		}
	}
	return nil
}

func (v *VideoLibrary) isLibraryFolderExist(recordedFolders []LibraryFolder, path string) bool {
	for _, f := range recordedFolders {
		if f.Path == path {
			return true
		}
	}
	return false
}

func (v *VideoLibrary) isVideoExist(recordedVideos []Video, path string) bool {
	for _, f := range recordedVideos {
		if f.Path == path {
			return true
		}
	}
	return false
}

func (v *VideoLibrary) GetFolderContent(relPath string) (LibraryFolder, error) {
	var folder LibraryFolder
	path := filepath.Join(v.Library.Path, relPath)

	result := v.Db.Preload("Childs").Preload("Videos").Where(&LibraryFolder{LibraryID: v.Library.ID, Path: path}).Find(&folder)
	return folder, result.Error
}

func (v *VideoLibrary) Update() error {
	validFolders := make([]string, 0)

	var recordedFolders []LibraryFolder
	var recordedVideos []Video
	result := v.Db.Where(&LibraryFolder{LibraryID: v.Library.ID}).Find(&recordedFolders)
	if result.Error != nil {
		return fmt.Errorf("unbale to get library folders: %v", result.Error)
	}
	result = v.Db.Model(&Video{}).Preload("LibraryFolder", "library_id = ?", v.Library.ID).Find(&recordedVideos)
	if result.Error != nil {
		return fmt.Errorf("unbale to get video medias: %v", result.Error)
	}

	return filepath.WalkDir(v.Library.Path, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			v.logger.Infof("Skip hidden folder %s", path)
			return filepath.SkipDir // skip hidden folder
		} else if path == v.Library.Path { // skip root folder
			return nil
		}

		v.logger.Debugf("Walk %s", path)
		// get recorded parent
		parent := v.getLibraryFolder(recordedFolders, filepath.Dir(path))
		if parent == nil {
			v.logger.Errorf("Unable to find parent %s", filepath.Dir(path))
			return fmt.Errorf("unable to find folder %s in the database", filepath.Dir(path))
		}

		if d.IsDir() && !v.isLibraryFolderExist(recordedFolders, path) {
			lFolder, err := v.createLibraryFolder(&parent.ID, path, d.Name())
			if err != nil {
				return err
			}
			recordedFolders = append(recordedFolders, *lFolder)
			validFolders = append(validFolders, filepath.Join(path, d.Name()))
		}

		if !d.IsDir() && IsValidVideoFile(d.Name()) {
			v.logger.Debugf("Find media file %s", d.Name())
			infos, err := ParseVideoMedia(path)
			if err != nil {
				v.logger.Warnf("Unable to parse %s file: %v", d.Name(), err)
				return nil
			}

			if !v.isVideoExist(recordedVideos, path) {
				duration, _ := strconv.ParseFloat(infos.Format.Duration, 32)
				videoModel := &Video{
					Name:            d.Name(),
					Path:            path,
					Duration:        duration,
					LibraryFolderID: parent.ID,
				}

				aIdx, sIdx := 1, 1
				for _, stream := range infos.Streams {
					switch stream.CodecType {
					case "audio":
						videoModel.AudioStreams = append(videoModel.AudioStreams, AudioStream{
							Lang: stream.Tags.Language,
							Idx:  aIdx,
						})
						aIdx++
					case "subtitle":
						videoModel.SubtitleStreams = append(videoModel.SubtitleStreams, SubtitleStream{
							Lang: stream.Tags.Language,
							Idx:  sIdx,
						})
						sIdx++
					}
				}

				if result := v.Db.Create(videoModel); result.Error != nil {
					v.logger.Warnf("Unable to recors video %s: %v", d.Name(), err)
				}
			}
		}

		return nil
	})
}

type LibraryFactory struct {
	Db        *gorm.DB
	Config    config.LibraryConfig
	Libraries map[string]VideoLibrary
}

func (f *LibraryFactory) GetVideoLibrary(name string) *VideoLibrary {
	for lName, library := range f.Libraries {
		if name == lName {
			return &library
		}
	}

	return nil
}

func (f *LibraryFactory) UpdateAllLibraries() error {
	g := new(errgroup.Group)
	g.SetLimit(10)

	for name, library := range f.Libraries {
		g.Go(func() error {
			logrus.Infof("Update %s library...", name)
			return library.Update()
		})
	}

	return g.Wait()
}

func (f *LibraryFactory) GetVideoById(videoid int) (*Video, error) {
	var vid Video

	result := f.Db.Preload("AudioStreams").Preload("SubtitleStreams").First(&vid, videoid)
	return &vid, result.Error
}

func InitLibraryFactory(config config.LibraryConfig) (*LibraryFactory, error) {
	db, err := gorm.Open(sqlite.Open(config.Database), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("Unable to open sqlite database: %v", err)
	}

	// AutoMigrate models
	db.AutoMigrate(&Library{})
	db.AutoMigrate(&LibraryFolder{})
	db.AutoMigrate(&Video{})
	db.AutoMigrate(&AudioStream{})
	db.AutoMigrate(&SubtitleStream{})

	// init video libraries
	libraries := make(map[string]VideoLibrary)
	for _, lConfig := range []struct {
		Name string
		Path string
	}{
		{
			Name: "movies",
			Path: config.Movies,
		},
		{
			Name: "tvshows",
			Path: config.Tvshows,
		},
	} {
		if !IsFolderExists(lConfig.Path) {
			return nil, fmt.Errorf("%s - Folder %s does not exist", lConfig.Name, lConfig.Path)
		}

		library := VideoLibrary{
			Db:     db,
			logger: logrus.WithField("library", lConfig.Name),
		}
		if err := library.Init(lConfig.Name, lConfig.Path); err != nil {
			return nil, fmt.Errorf("unable to init %s library: %v", lConfig.Name, err)
		}
		libraries[lConfig.Name] = library
	}

	return &LibraryFactory{
		Db:        db,
		Config:    config,
		Libraries: libraries,
	}, nil
}
