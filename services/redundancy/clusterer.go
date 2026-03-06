package main

// Cluster represents a group of semantically similar TDUs.
type Cluster struct {
	ID      int          // cluster index
	Members []*TDURecord // TDU records in this cluster
	Domain  string       // dominant domain (from majority of members)
}

// Clusterer groups TDUs by semantic similarity using SimHash hamming distance.
type Clusterer struct {
	threshold float64 // cosine similarity threshold (e.g. 0.85)
	maxDist   int     // max hamming distance derived from threshold
}

// NewClusterer creates a Clusterer with the given similarity threshold.
// Threshold is cosine similarity in [0.0, 1.0]; internally converted to
// hamming distance: maxDist = floor(64 * (1 - threshold)).
func NewClusterer(threshold float64) *Clusterer {
	if threshold < 0 {
		threshold = 0
	}
	if threshold > 1.0 {
		threshold = 1.0
	}
	maxDist := int(64.0 * (1.0 - threshold))
	return &Clusterer{
		threshold: threshold,
		maxDist:   maxDist,
	}
}

// ClusterTDUs groups TDU records into clusters. Uses single-linkage:
// a TDU joins the first cluster where it is similar to any existing member.
// This is O(n^2) but sufficient for typical dataset sizes.
func (c *Clusterer) ClusterTDUs(records []TDURecord) []Cluster {
	if len(records) == 0 {
		return nil
	}

	// assigned[i] = cluster index, -1 = unassigned
	assigned := make([]int, len(records))
	for i := range assigned {
		assigned[i] = -1
	}

	var clusters []Cluster

	for i := range records {
		if assigned[i] != -1 {
			continue
		}

		// Start a new cluster with this TDU
		clusterIdx := len(clusters)
		cluster := Cluster{
			ID:      clusterIdx,
			Members: []*TDURecord{&records[i]},
		}
		assigned[i] = clusterIdx

		// Find all unassigned TDUs similar to any member of this cluster
		// Iterate until no new members added (transitive closure)
		changed := true
		for changed {
			changed = false
			for j := range records {
				if assigned[j] != -1 {
					continue
				}
				if c.isSimilarToCluster(&records[j], cluster.Members) {
					cluster.Members = append(cluster.Members, &records[j])
					assigned[j] = clusterIdx
					changed = true
				}
			}
		}

		cluster.Domain = dominantDomain(cluster.Members)
		clusters = append(clusters, cluster)
	}

	return clusters
}

// isSimilarToCluster checks if a TDU is within threshold of any cluster member.
func (c *Clusterer) isSimilarToCluster(tdu *TDURecord, members []*TDURecord) bool {
	for _, m := range members {
		dist := hammingDistance(tdu.simHash, m.simHash)
		if dist <= c.maxDist {
			return true
		}
	}
	return false
}

// dominantDomain returns the most common domain among cluster members.
func dominantDomain(members []*TDURecord) string {
	counts := make(map[string]int)
	for _, m := range members {
		if m.Domain != "" {
			counts[m.Domain]++
		}
	}
	var best string
	var bestCount int
	for d, cnt := range counts {
		if cnt > bestCount {
			best = d
			bestCount = cnt
		}
	}
	return best
}
