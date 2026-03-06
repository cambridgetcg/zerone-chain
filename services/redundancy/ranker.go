package main

// RankedCluster holds the ranking result for a single cluster.
type RankedCluster struct {
	Cluster   Cluster
	Canonical *TDURecord   // the best TDU in the cluster
	Redundant []*TDURecord // all non-canonical members
}

// Ranker selects the canonical TDU within each cluster.
type Ranker struct{}

// NewRanker creates a new Ranker.
func NewRanker() *Ranker {
	return &Ranker{}
}

// RankClusters selects a canonical TDU for each multi-member cluster.
// Single-member clusters have no redundancy and are returned with nil Redundant.
//
// Canonical selection rules (in priority order):
// 1. Correction TDUs always win over non-corrections
// 2. Highest fitness score wins
// 3. Oldest TDU (lowest CreatedAt) wins ties
func (r *Ranker) RankClusters(clusters []Cluster) []RankedCluster {
	ranked := make([]RankedCluster, 0, len(clusters))

	for _, c := range clusters {
		rc := RankedCluster{Cluster: c}

		if len(c.Members) <= 1 {
			if len(c.Members) == 1 {
				rc.Canonical = c.Members[0]
			}
			ranked = append(ranked, rc)
			continue
		}

		// Find canonical
		canonical := c.Members[0]
		for _, m := range c.Members[1:] {
			if r.beats(m, canonical) {
				canonical = m
			}
		}

		rc.Canonical = canonical
		rc.Redundant = make([]*TDURecord, 0, len(c.Members)-1)
		for _, m := range c.Members {
			if m != canonical {
				rc.Redundant = append(rc.Redundant, m)
			}
		}

		ranked = append(ranked, rc)
	}

	return ranked
}

// beats returns true if challenger should be canonical over current.
func (r *Ranker) beats(challenger, current *TDURecord) bool {
	// Rule 1: corrections always win over non-corrections
	if challenger.IsCorrection && !current.IsCorrection {
		return true
	}
	if !challenger.IsCorrection && current.IsCorrection {
		return false
	}

	// Rule 2: higher fitness score wins
	if challenger.FitnessScore > current.FitnessScore {
		return true
	}
	if challenger.FitnessScore < current.FitnessScore {
		return false
	}

	// Rule 3: older TDU wins ties (lower CreatedAt)
	if challenger.CreatedAt > 0 && current.CreatedAt > 0 {
		return challenger.CreatedAt < current.CreatedAt
	}

	return false
}
