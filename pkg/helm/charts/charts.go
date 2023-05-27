package charts

import (
	"errors"
	"sync"

	"github.com/go-logr/logr"
)

// Charts returns the global Charts. This collection is created
// once and is accessible globally.
//
// Do not change the content of this collection directly.
func Charts() Collection {
	return *globalCollection
}

// LoadCharts uses the provided options to populate the existing
// Charts into the global Chart collection.
//
// Call this function only once when the controller initializes.
func LoadCharts(logger logr.Logger, path string) error {
	collectionMutex.Lock()
	defer collectionMutex.Unlock()

	if len(*globalCollection) > 0 {
		return errors.New("collection is not empty")
	}

	loader := &DirectoryLoader{
		Logger:       logger,
		Path:         path,
		FilePatterns: []string{"*.tgz"},
		Collection:   globalCollection,
	}

	return loader.Load()
}

var (
	collectionMutex  sync.Mutex
	globalCollection *Collection = &Collection{}
)
