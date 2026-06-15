// v2_meta.go — GET /api/v2/meta
//
// The /api/v2/meta endpoint is the bootstrap contract for WebUI v2.
// It returns:
//   - version: the daemon build version string
//   - api_versions: the API surfaces this daemon supports (always includes "v1", "v2")
//   - groups: slice of registered group slugs (empty slice when no groups exist)
//
// The WebUI v2 calls this once on mount (staleTime=Infinity in TanStack Query)
// to discover what the daemon supports before rendering any screen.

package dashboard

import (
	"net/http"

	"github.com/cajasmota/grafel/internal/version"
)

// v2MetaReply is the data payload inside the v2 envelope for /api/v2/meta.
type v2MetaReply struct {
	// Version is the daemon build version (e.g. "1.2.3", "0.0.0-dev").
	Version string `json:"version"`
	// APIVersions lists the API surfaces supported by this daemon binary.
	// Always contains at least ["v1", "v2"].
	APIVersions []string `json:"api_versions"`
	// Groups is the list of registered group slugs. The WebUI v2 uses this
	// to decide whether to show the onboarding wizard or the main graph.
	Groups []string `json:"groups"`
}

// handleV2Meta — GET /api/v2/meta
func (s *Server) handleV2Meta(w http.ResponseWriter, r *http.Request) {
	groups, err := s.registry.ListGroups()
	if err != nil {
		writeV2Err(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	slugs := make([]string, 0, len(groups))
	for _, g := range groups {
		slugs = append(slugs, g.Name)
	}
	reply := v2MetaReply{
		Version:     version.Version,
		APIVersions: []string{"v1", "v2"},
		Groups:      slugs,
	}
	writeV2JSON(w, http.StatusOK, v2OK(reply))
}
