package contentdb

// Package represents a ContentDB package (mod, game, or texture pack).
type Package struct {
	Author           string   `json:"author"`
	Name             string   `json:"name"`
	Title            string   `json:"title"`
	ShortDescription string   `json:"short_description"`
	Type             string   `json:"type"`
	Score            float64  `json:"score"`
	Downloads        int      `json:"downloads"`
	Thumbnail        string   `json:"thumbnail"`
	Tags             []string `json:"tags"`
	License          string   `json:"license"`
	Repo             string   `json:"repo"`
	// Provides lists every mod name shipped by this package.
	// For modpacks this enumerates all sub-mod directory names.
	Provides []string `json:"provides"`
}

// Dependency describes one dependency entry for a package.
type Dependency struct {
	IsOptional bool     `json:"is_optional"`
	Name       string   `json:"name"`
	Packages   []string `json:"packages"`
}

// DependenciesResponse is returned by the ContentDB /dependencies/ endpoint.
// Each key is an "author/name" package ID; the value is that package's
// dependency list.  The response covers the full transitive dep graph.
type DependenciesResponse map[string][]Dependency

// PackageListResponse is returned by paginated list endpoints.
type PackageListResponse struct {
	Page      int       `json:"page"`
	PerPage   int       `json:"per_page"`
	PageCount int       `json:"page_count"`
	Total     int       `json:"total"`
	Items     []Package `json:"items"`
}

// Release represents a single ContentDB release for a package.
type Release struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	ReleaseDate string `json:"release_date"`
	URL         string `json:"url"`
	Downloads   int    `json:"downloads"`
}
