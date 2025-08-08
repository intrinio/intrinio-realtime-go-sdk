package composite

// DataCacheFactory provides factory methods for creating data caches
type DataCacheFactory struct{}

// Create creates a new DataCache instance
func (f *DataCacheFactory) Create() DataCache {
	return NewDataCache()
}

// Create is a convenience function to create a new DataCache
func CreateDataCache() DataCache {
	return NewDataCache()
} 