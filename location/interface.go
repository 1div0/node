package location

// Resolver allows resolving location by ip
type Resolver interface {
	ResolveCountry(ip string) (string, error)
}

// Detector allows detecting location by current ip
type Detector interface {
	DetectLocation() (Location, error)
}

// Cache allows caching location
type Cache interface {
	Get() Location
	Set(Location)
}
