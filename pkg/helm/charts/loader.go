package charts

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/chart/loader"
)

type DirectoryLoader struct {
	Logger       logr.Logger
	Path         string
	FilePatterns []string
	Collection   *Collection
}

func (l *DirectoryLoader) Load() error {
	if err := filepath.WalkDir(l.Path, l.processDirEntry); err != nil {
		l.Logger.V(2).Error(err, "unable to walk SearchPath")
	}

	if l.Collection.Empty() {
		return fmt.Errorf("unable to find any charts in search paths %s", l.Path)
	}

	return nil
}

func (l *DirectoryLoader) processDirEntry(path string, d fs.DirEntry, e error) error {
	if e != nil {
		l.Logger.V(2).Info("error occurred while searching directory",
			"path", path, "error", e)

		return nil
	}

	if d.IsDir() {
		return l.tryEntryAsChart(path, true)
	} else {
		for _, pattern := range l.FilePatterns {
			matched, err := filepath.Match(pattern, d.Name())

			if matched && err == nil {
				l.Logger.V(2).Info("found a matching file",
					"path", path)

				return l.tryEntryAsChart(path, false)
			}

			if err != nil {
				l.Logger.V(2).Error(err,
					"error occurred while matching file name to the pattern",
					"path", path,
					"pattern", pattern)
			}
		}
	}

	return nil
}

func (l *DirectoryLoader) tryEntryAsChart(path string, isDir bool) error {
	l.Logger.V(2).Info("trying entry as chart",
		"path", path,
		"isDirectory", isDir)

	chart, err := loader.Load(path)
	if err != nil {
		l.Logger.V(2).Info("entry does not contain a chart",
			"path", path,
			"isDirectory", isDir,
			"error", err)
	} else {
		l.Logger.V(2).Info("entry contains a chart",
			"path", path,
			"isDirectory", isDir)

		l.Collection.Append(chart)

		l.Logger.Info("chart added to the collection",
			"chartName", chart.Metadata.Name,
			"chartVersion", chart.Metadata.Version)

		if isDir {
			return filepath.SkipDir
		}
	}

	return nil
}
