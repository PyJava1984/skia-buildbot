// paramsets keeps a running summary of paramsets per test, digest pair.
package paramsets

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
)

// ParamSummary keep precalculated paramsets for each test, digest pair.
// It is not thread safe. The client of this package needs to make sure there
// are no conflicts.
type ParamSummary struct {
	// map [test]map[digest] paramset.
	byTrace               map[string]map[string]paramtools.ParamSet
	byTraceIncludeIgnored map[string]map[string]paramtools.ParamSet
}

// byTraceForTile calculates all the paramsets from the given tile and tallies.
func byTraceForTile(tile *tiling.Tile, traceTally map[string]tally.Tally) map[string]map[string]paramtools.ParamSet {
	ret := map[string]map[string]paramtools.ParamSet{}

	for id, t := range traceTally {
		if tr, ok := tile.Traces[id]; ok {
			test := tr.Params()[types.PRIMARY_KEY_FIELD]
			for digest := range t {
				if foundTest, ok := ret[test]; !ok {
					ret[test] = map[string]paramtools.ParamSet{digest: paramtools.NewParamSet(tr.Params())}
				} else if foundDigest, ok := foundTest[digest]; !ok {
					foundTest[digest] = paramtools.NewParamSet(tr.Params())
				} else {
					foundDigest.AddParams(tr.Params())
				}
			}
		}
	}
	return ret
}

// oneStep does a single step, calculating all the paramsets from the latest tile and tallies.
//
// Returns the paramsets for both the tile with and without ignored traces included.
func oneStep(tilePair *types.TilePair, tallies *tally.Tallies) (map[string]map[string]paramtools.ParamSet, map[string]map[string]paramtools.ParamSet) {
	defer timer.New("paramsets").Stop()
	return byTraceForTile(tilePair.Tile, tallies.ByTrace()), byTraceForTile(tilePair.TileWithIgnores, tallies.ByTrace())
}

// New creates a new ParamSummary.
func New() *ParamSummary {
	return &ParamSummary{}
}

// Calculate sets the values the ParamSummary based on the given tile.
func (s *ParamSummary) Calculate(tilePair *types.TilePair, tallies *tally.Tallies) {
	s.byTrace, s.byTraceIncludeIgnored = oneStep(tilePair, tallies)
}

// Get returns the paramset for the given digest. If 'include' is true
// then the paramset is calculated including ignored traces.
func (s *ParamSummary) Get(test, digest string, include bool) map[string][]string {
	useMap := s.byTrace
	if include {
		useMap = s.byTraceIncludeIgnored
	}

	if foundTest, ok := useMap[test]; ok {
		return foundTest[digest]
	}
	return nil
}

// GetByTest returns the parameter sets organized by tests and digests:
//      map[test_name]map[digest]ParamSet
func (s *ParamSummary) GetByTest(includeIngores bool) map[string]map[string]paramtools.ParamSet {
	if includeIngores {
		return s.byTraceIncludeIgnored
	}
	return s.byTrace
}
