// Package main implements a knowledge context server that bridges
// the ZERONE Tree of Knowledge to AI agent prompts.
//
// It exposes a single endpoint that queries on-chain facts,
// filters by domain/confidence/status, and returns formatted
// context blocks ready for prompt injection.
//
// Usage:
//
//	go run . --node http://localhost:1317 --port 8222
//	curl "http://localhost:8222/context?domains=physics,mathematics&min_confidence=50&format=xml"
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	nodeURL = flag.String("node", "http://localhost:1317", "ZERONE node REST endpoint")
	port    = flag.Int("port", 8222, "Server port")
)

var demandBuffer struct {
	mu      sync.Mutex
	reports map[string]*demandReport
}

type demandReport struct {
	Domain      string
	Subject     string
	Queries     uint64
	Fulfilled   uint64
	Unfulfilled uint64
}

func init() {
	demandBuffer.reports = make(map[string]*demandReport)
}

func trackDemand(domain, subject string, factCount int) {
	demandBuffer.mu.Lock()
	defer demandBuffer.mu.Unlock()

	key := domain + "|" + subject
	report, ok := demandBuffer.reports[key]
	if !ok {
		report = &demandReport{Domain: domain, Subject: subject}
		demandBuffer.reports[key] = report
	}
	report.Queries++
	if factCount == 0 {
		report.Unfulfilled++
	} else {
		report.Fulfilled++
	}
}

// ─── On-chain types ──────────────────────────────────────────────────────────

type ClaimStructure struct {
	Subject       string   `json:"subject,omitempty"`
	Predicate     string   `json:"predicate,omitempty"`
	Object        string   `json:"object,omitempty"`
	Scope         string   `json:"scope,omitempty"`
	TemporalScope string   `json:"temporal_scope,omitempty"`
	Negatable     bool     `json:"negatable,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}

type Fact struct {
	ID            string          `json:"id"`
	Content       string          `json:"content"`
	Domain        string          `json:"domain"`
	Category      string          `json:"category"`
	Confidence    string          `json:"confidence"`
	Status        string          `json:"status"`
	Submitter     string          `json:"submitter"`
	Stratum       string          `json:"stratum,omitempty"`
	References    []string        `json:"references,omitempty"`
	ClaimID       string          `json:"claim_id,omitempty"`
	ClaimType     string          `json:"claim_type,omitempty"`
	Structure     *ClaimStructure `json:"structure,omitempty"`
	CanonicalForm string          `json:"canonical_form,omitempty"`
	CanonicalHash string          `json:"canonical_hash,omitempty"`
	FitnessScore  string          `json:"fitness_score,omitempty"`
	Energy        string          `json:"energy,omitempty"`
	EnergyCap     string          `json:"energy_cap,omitempty"`
	// Niche fields
	NicheKey      string `json:"niche_key,omitempty"`
	NicheLeader   bool   `json:"niche_leader,omitempty"`
	NicheRank     string `json:"niche_rank,omitempty"`
	NicheSize     string `json:"niche_size,omitempty"`
	CompetitionTax string `json:"competition_tax,omitempty"`
	// Lineage fields
	ParentFactId  string   `json:"parent_fact_id,omitempty"`
	ChildFactIds  []string `json:"child_fact_ids,omitempty"`
	LineageDepth  string   `json:"lineage_depth,omitempty"`
	ProgenyCount  string   `json:"progeny_count,omitempty"`
	LineageRootId string   `json:"lineage_root_id,omitempty"`
}

type FactRelation struct {
	SourceFactId   string `json:"source_fact_id"`
	TargetFactId   string `json:"target_fact_id"`
	Relation       string `json:"relation"`
	CreatedAtBlock string `json:"created_at_block"`
	Creator        string `json:"creator"`
}

type FactRelationsResponse struct {
	Relations []FactRelation `json:"relations"`
}

type FactsResponse struct {
	Facts []Fact `json:"facts"`
}

type Domain struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type DomainsResponse struct {
	Domains []Domain `json:"domains"`
}

// ─── Status mapping ──────────────────────────────────────────────────────────

var statusToHuman = map[string]string{
	"FACT_STATUS_UNSPECIFIED": "unspecified",
	"FACT_STATUS_PENDING":    "pending",
	"FACT_STATUS_PROVISIONAL": "provisional",
	"FACT_STATUS_VERIFIED":   "verified",
	"FACT_STATUS_ACTIVE":     "active",
	"FACT_STATUS_CONTESTED":  "contested",
	"FACT_STATUS_CHALLENGED": "challenged",
	"FACT_STATUS_SUPERSEDED": "superseded",
	"FACT_STATUS_EXPIRED":    "expired",
	"FACT_STATUS_DISPROVEN":  "disproven",
}

var trustedStatuses = map[string]bool{
	"FACT_STATUS_VERIFIED": true,
	"FACT_STATUS_ACTIVE":   true,
}

var allNonTerminalStatuses = map[string]bool{
	"FACT_STATUS_VERIFIED":   true,
	"FACT_STATUS_ACTIVE":     true,
	"FACT_STATUS_CONTESTED":  true,
	"FACT_STATUS_CHALLENGED": true,
	"FACT_STATUS_PROVISIONAL": true,
}

// ─── Fetchers ────────────────────────────────────────────────────────────────

func fetchFacts() ([]Fact, error) {
	resp, err := http.Get(*nodeURL + "/zerone/knowledge/v1/facts")
	if err != nil {
		return nil, fmt.Errorf("node unreachable: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var fr FactsResponse
	if err := json.Unmarshal(body, &fr); err != nil {
		return nil, fmt.Errorf("bad response: %w", err)
	}
	return fr.Facts, nil
}

// relationTypeToHuman maps protobuf enum names to short human-readable strings.
var relationTypeToHuman = map[string]string{
	"RELATION_TYPE_UNSPECIFIED": "unspecified",
	"RELATION_TYPE_SUPPORTS":   "supports",
	"RELATION_TYPE_CONTRADICTS": "contradicts",
	"RELATION_TYPE_REQUIRES":   "requires",
	"RELATION_TYPE_REFINES":    "refines",
	"RELATION_TYPE_GENERALIZES": "generalizes",
	"RELATION_TYPE_SUPERSEDES": "supersedes",
}

func humanRelationType(rt string) string {
	if h, ok := relationTypeToHuman[rt]; ok {
		return h
	}
	return rt
}

func fetchFactRelations(factID, direction string) ([]FactRelation, error) {
	url := fmt.Sprintf("%s/zerone/knowledge/v1/facts/%s/relations?direction=%s", *nodeURL, factID, direction)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("node unreachable: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var fr FactRelationsResponse
	if err := json.Unmarshal(body, &fr); err != nil {
		return nil, fmt.Errorf("bad response: %w", err)
	}
	return fr.Relations, nil
}

func fetchDomains() ([]Domain, error) {
	resp, err := http.Get(*nodeURL + "/zerone/knowledge/v1/domains")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var dr DomainsResponse
	json.Unmarshal(body, &dr)
	return dr.Domains, nil
}

// ─── Filtering ───────────────────────────────────────────────────────────────

// claimTypeToHuman maps protobuf enum names to short human-readable strings.
var claimTypeToHuman = map[string]string{
	"CLAIM_TYPE_UNSPECIFIED": "assertion",
	"CLAIM_TYPE_ASSERTION":  "assertion",
	"CLAIM_TYPE_RELATION":   "relation",
	"CLAIM_TYPE_DEFINITION": "definition",
	"CLAIM_TYPE_CONSTRAINT": "constraint",
	"CLAIM_TYPE_NEGATION":   "negation",
	"CLAIM_TYPE_OBSERVATION": "observation",
}

func humanClaimType(ct string) string {
	if h, ok := claimTypeToHuman[ct]; ok {
		return h
	}
	if ct == "" || ct == "0" {
		return "assertion"
	}
	return ct
}

func filterFacts(facts []Fact, domains map[string]bool, minConf float64, includeChallenged bool, claimTypes map[string]bool) []Fact {
	allowed := trustedStatuses
	if includeChallenged {
		allowed = allNonTerminalStatuses
	}

	var out []Fact
	for _, f := range facts {
		if !allowed[f.Status] {
			continue
		}
		conf := parseConfidence(f.Confidence)
		if conf < minConf {
			continue
		}
		if len(domains) > 0 && !domains[f.Domain] {
			continue
		}
		if len(claimTypes) > 0 && !claimTypes[humanClaimType(f.ClaimType)] {
			continue
		}
		out = append(out, f)
	}

	sort.Slice(out, func(i, j int) bool {
		ci := parseConfidence(out[i].Confidence)
		cj := parseConfidence(out[j].Confidence)
		if ci != cj {
			return ci > cj
		}
		return out[i].Domain < out[j].Domain
	})
	return out
}

func parseConfidence(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v / 10000 // BPS → percentage
}

func parseFitness(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func fitnessLabel(score float64) string {
	switch {
	case score >= 800000:
		return "keystone"
	case score >= 600000:
		return "thriving"
	case score >= 300000:
		return "healthy"
	case score >= 100000:
		return "low"
	default:
		return "critical"
	}
}

// ─── Formatters ──────────────────────────────────────────────────────────────

func formatXML(facts []Fact, query string, showCanonical bool) string {
	var b strings.Builder
	b.WriteString("<knowledge_context>\n")
	b.WriteString(fmt.Sprintf("  <source>ZERONE Tree of Knowledge</source>\n"))
	b.WriteString(fmt.Sprintf("  <retrieved>%s</retrieved>\n", time.Now().UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("  <fact_count>%d</fact_count>\n", len(facts)))

	for _, f := range facts {
		status := statusToHuman[f.Status]
		if status == "" {
			status = f.Status
		}
		conf := parseConfidence(f.Confidence)
		ct := humanClaimType(f.ClaimType)
		fitness := parseFitness(f.FitnessScore)
		fl := fitnessLabel(fitness)
		energyAttr := ""
		if f.Energy != "" && f.Energy != "0" {
			energyAttr = fmt.Sprintf(" energy=\"%s/%s\"", f.Energy, f.EnergyCap)
		}
		nicheAttr := ""
		if f.NicheRank != "" && f.NicheRank != "0" {
			nicheAttr = fmt.Sprintf(" niche_rank=\"%s\" niche_size=\"%s\"", f.NicheRank, f.NicheSize)
			if f.NicheLeader {
				nicheAttr += " niche_leader=\"true\""
			}
		}
		lineageAttr := ""
		if f.ParentFactId != "" {
			lineageAttr += fmt.Sprintf(" parent=\"%s\"", f.ParentFactId)
		}
		if len(f.ChildFactIds) > 0 {
			lineageAttr += fmt.Sprintf(" children=\"%d\"", len(f.ChildFactIds))
		}
		if f.ProgenyCount != "" && f.ProgenyCount != "0" {
			lineageAttr += fmt.Sprintf(" progeny=\"%s\"", f.ProgenyCount)
		}
		if f.LineageDepth != "" && f.LineageDepth != "0" {
			lineageAttr += fmt.Sprintf(" lineage_depth=\"%s\"", f.LineageDepth)
		}
		if f.LineageRootId != "" {
			lineageAttr += fmt.Sprintf(" lineage_root=\"%s\"", f.LineageRootId)
		}
		b.WriteString(fmt.Sprintf("  <fact id=\"%s\" domain=\"%s\" confidence=\"%.1f%%\" status=\"%s\" category=\"%s\" type=\"%s\" fitness=\"%.0f\" fitness_label=\"%s\"%s%s%s>\n",
			f.ID, f.Domain, conf, status, f.Category, ct, fitness, fl, energyAttr, nicheAttr, lineageAttr))
		b.WriteString(fmt.Sprintf("    <content>%s</content>\n", f.Content))
		if f.Structure != nil {
			b.WriteString("    <structure>\n")
			if f.Structure.Subject != "" {
				b.WriteString(fmt.Sprintf("      <subject>%s</subject>\n", f.Structure.Subject))
			}
			if f.Structure.Predicate != "" {
				b.WriteString(fmt.Sprintf("      <predicate>%s</predicate>\n", f.Structure.Predicate))
			}
			if f.Structure.Object != "" {
				b.WriteString(fmt.Sprintf("      <object>%s</object>\n", f.Structure.Object))
			}
			if f.Structure.Scope != "" {
				b.WriteString(fmt.Sprintf("      <scope>%s</scope>\n", f.Structure.Scope))
			}
			if f.Structure.TemporalScope != "" {
				b.WriteString(fmt.Sprintf("      <temporal_scope>%s</temporal_scope>\n", f.Structure.TemporalScope))
			}
			if len(f.Structure.Tags) > 0 {
				b.WriteString(fmt.Sprintf("      <tags>%s</tags>\n", strings.Join(f.Structure.Tags, ",")))
			}
			b.WriteString("    </structure>\n")
		}
		if showCanonical && f.CanonicalForm != "" {
			b.WriteString(fmt.Sprintf("    <canonical>%s</canonical>\n", f.CanonicalForm))
		}
		if len(f.References) > 0 {
			b.WriteString(fmt.Sprintf("    <references>%s</references>\n", strings.Join(f.References, ",")))
		}
		// Include typed relations if available
		if rels, err := fetchFactRelations(f.ID, "outgoing"); err == nil && len(rels) > 0 {
			for _, rel := range rels {
				rt := humanRelationType(rel.Relation)
				b.WriteString(fmt.Sprintf("    <%s>%s</%s>\n", rt, rel.TargetFactId, rt))
			}
		}
		b.WriteString("  </fact>\n")
	}

	b.WriteString("</knowledge_context>")

	if query != "" {
		return fmt.Sprintf("The following verified knowledge is sourced from the ZERONE blockchain Tree of Knowledge. "+
			"Each fact has been verified through stake-weighted consensus. Confidence scores reflect verification strength. "+
			"Challenged/contested facts are included for context but should be treated as disputed.\n\n%s\n\n"+
			"Using the above as grounding context, respond to:\n%s", b.String(), query)
	}
	return b.String()
}

func formatJSON(facts []Fact) string {
	type relationsOut struct {
		Supports    []string `json:"supports,omitempty"`
		Contradicts []string `json:"contradicts,omitempty"`
		Requires    []string `json:"requires,omitempty"`
		Refines     []string `json:"refines,omitempty"`
		Generalizes []string `json:"generalizes,omitempty"`
		Supersedes  []string `json:"supersedes,omitempty"`
	}
	type structureOut struct {
		Subject       string   `json:"subject,omitempty"`
		Predicate     string   `json:"predicate,omitempty"`
		Object        string   `json:"object,omitempty"`
		Scope         string   `json:"scope,omitempty"`
		TemporalScope string   `json:"temporal_scope,omitempty"`
		Negatable     bool     `json:"negatable,omitempty"`
		Tags          []string `json:"tags,omitempty"`
	}
	type factOut struct {
		ID            string        `json:"id"`
		Domain        string        `json:"domain"`
		Content       string        `json:"content"`
		ConfidencePct float64       `json:"confidence_pct"`
		FitnessScore  float64       `json:"fitness_score"`
		FitnessPct    float64       `json:"fitness_pct"`
		FitnessLabel  string        `json:"fitness_label"`
		Status        string        `json:"status"`
		Category      string        `json:"category"`
		ClaimType     string        `json:"claim_type"`
		CanonicalForm string        `json:"canonical_form,omitempty"`
		CanonicalHash string        `json:"canonical_hash,omitempty"`
		References    []string      `json:"references,omitempty"`
		Structure     *structureOut `json:"structure,omitempty"`
		Relations     *relationsOut `json:"relations,omitempty"`
		Energy        uint64        `json:"energy,omitempty"`
		EnergyCap     uint64        `json:"energy_cap,omitempty"`
		EnergyPct     float64       `json:"energy_pct,omitempty"`
		// Niche
		NicheKey       string `json:"niche_key,omitempty"`
		NicheLeader    bool   `json:"niche_leader,omitempty"`
		NicheRank      uint64 `json:"niche_rank,omitempty"`
		NicheSize      uint64 `json:"niche_size,omitempty"`
		CompetitionTax uint64 `json:"competition_tax,omitempty"`
		// Lineage
		ParentFactId  string   `json:"parent_fact_id,omitempty"`
		ChildFactIds  []string `json:"child_fact_ids,omitempty"`
		LineageDepth  uint64   `json:"lineage_depth,omitempty"`
		ProgenyCount  uint64   `json:"progeny_count,omitempty"`
		LineageRootId string   `json:"lineage_root_id,omitempty"`
	}
	type output struct {
		Source    string    `json:"source"`
		Retrieved string   `json:"retrieved"`
		FactCount int      `json:"fact_count"`
		Facts     []factOut `json:"facts"`
	}

	o := output{
		Source:    "zerone_tree_of_knowledge",
		Retrieved: time.Now().UTC().Format(time.RFC3339),
		FactCount: len(facts),
	}
	for _, f := range facts {
		status := statusToHuman[f.Status]
		if status == "" {
			status = f.Status
		}
		fitness := parseFitness(f.FitnessScore)
		energy, _ := strconv.ParseUint(f.Energy, 10, 64)
		energyCap, _ := strconv.ParseUint(f.EnergyCap, 10, 64)
		var energyPct float64
		if energyCap > 0 {
			energyPct = float64(energy) / float64(energyCap) * 100.0
		}
		fo := factOut{
			ID:            f.ID,
			Domain:        f.Domain,
			Content:       f.Content,
			ConfidencePct: parseConfidence(f.Confidence),
			FitnessScore:  fitness,
			FitnessPct:    fitness / 10000, // BPS → percentage
			FitnessLabel:  fitnessLabel(fitness),
			Status:        status,
			Category:      f.Category,
			ClaimType:     humanClaimType(f.ClaimType),
			CanonicalForm: f.CanonicalForm,
			CanonicalHash: f.CanonicalHash,
			References:    f.References,
			Energy:        energy,
			EnergyCap:     energyCap,
			EnergyPct:     energyPct,
			// Niche
			NicheKey:    f.NicheKey,
			NicheLeader: f.NicheLeader,
			// Lineage
			ParentFactId:  f.ParentFactId,
			ChildFactIds:  f.ChildFactIds,
			LineageRootId: f.LineageRootId,
		}
		if nr, err := strconv.ParseUint(f.NicheRank, 10, 64); err == nil {
			fo.NicheRank = nr
		}
		if ns, err := strconv.ParseUint(f.NicheSize, 10, 64); err == nil {
			fo.NicheSize = ns
		}
		if ct, err := strconv.ParseUint(f.CompetitionTax, 10, 64); err == nil {
			fo.CompetitionTax = ct
		}
		if d, err := strconv.ParseUint(f.LineageDepth, 10, 64); err == nil {
			fo.LineageDepth = d
		}
		if p, err := strconv.ParseUint(f.ProgenyCount, 10, 64); err == nil {
			fo.ProgenyCount = p
		}
		if f.Structure != nil {
			fo.Structure = &structureOut{
				Subject:       f.Structure.Subject,
				Predicate:     f.Structure.Predicate,
				Object:        f.Structure.Object,
				Scope:         f.Structure.Scope,
				TemporalScope: f.Structure.TemporalScope,
				Negatable:     f.Structure.Negatable,
				Tags:          f.Structure.Tags,
			}
		}
		// Fetch and group typed relations
		if rels, err := fetchFactRelations(f.ID, "outgoing"); err == nil && len(rels) > 0 {
			ro := &relationsOut{}
			for _, rel := range rels {
				rt := humanRelationType(rel.Relation)
				switch rt {
				case "supports":
					ro.Supports = append(ro.Supports, rel.TargetFactId)
				case "contradicts":
					ro.Contradicts = append(ro.Contradicts, rel.TargetFactId)
				case "requires":
					ro.Requires = append(ro.Requires, rel.TargetFactId)
				case "refines":
					ro.Refines = append(ro.Refines, rel.TargetFactId)
				case "generalizes":
					ro.Generalizes = append(ro.Generalizes, rel.TargetFactId)
				case "supersedes":
					ro.Supersedes = append(ro.Supersedes, rel.TargetFactId)
				}
			}
			fo.Relations = ro
		}
		o.Facts = append(o.Facts, fo)
	}
	data, _ := json.MarshalIndent(o, "", "  ")
	return string(data)
}

// ─── OpenAI-compatible tool format ───────────────────────────────────────────

type ToolResponse struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func formatToolResponse(facts []Fact) string {
	ctx := formatXML(facts, "", false)
	tr := ToolResponse{Role: "tool", Content: ctx}
	data, _ := json.Marshal(tr)
	return string(data)
}

// matchesAnyTag returns true if any of the fact's tags match any of the filter tags (OR match).
func matchesAnyTag(factTags []string, filterTags []string) bool {
	for _, ft := range factTags {
		normalized := strings.ToLower(strings.TrimSpace(ft))
		for _, filter := range filterTags {
			if normalized == filter {
				return true
			}
		}
	}
	return false
}

// ─── HTTP handler ────────────────────────────────────────────────────────────

func contextHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Parse domain filter
	domains := make(map[string]bool)
	if d := q.Get("domains"); d != "" {
		for _, dom := range strings.Split(d, ",") {
			domains[strings.TrimSpace(dom)] = true
		}
	}

	// Parse confidence threshold (percentage, default 50%)
	minConf := 50.0
	if mc := q.Get("min_confidence"); mc != "" {
		if v, err := strconv.ParseFloat(mc, 64); err == nil {
			minConf = v
		}
	}

	// Include challenged facts?
	includeChallenged := q.Get("include_challenged") == "true" || q.Get("include_challenged") == "1"

	// Parse claim type filter
	claimTypes := make(map[string]bool)
	if t := q.Get("type"); t != "" {
		for _, ct := range strings.Split(t, ",") {
			claimTypes[strings.TrimSpace(ct)] = true
		}
	}

	// Format
	format := q.Get("format")
	if format == "" {
		format = "xml"
	}

	// Optional query for prompt wrapping
	query := q.Get("query")

	// Include canonical forms in output?
	showCanonical := q.Get("canonical") == "true" || q.Get("canonical") == "1"

	// Fitness filter and sort
	sortBy := q.Get("sort") // "fitness" or default (confidence)
	minFitness := 0.0
	if mf := q.Get("min_fitness"); mf != "" {
		if v, err := strconv.ParseFloat(mf, 64); err == nil {
			minFitness = v
		}
	}

	// Subject and tag filters (structured claims)
	subjectFilter := strings.ToLower(strings.TrimSpace(q.Get("subject")))
	var tagFilters []string
	if t := q.Get("tags"); t != "" {
		for _, tag := range strings.Split(t, ",") {
			tagFilters = append(tagFilters, strings.ToLower(strings.TrimSpace(tag)))
		}
	}

	// Fetch and filter
	facts, err := fetchFacts()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err), http.StatusBadGateway)
		return
	}

	filtered := filterFacts(facts, domains, minConf, includeChallenged, claimTypes)

	// Track demand for each queried domain/subject
	for dom := range domains {
		trackDemand(dom, subjectFilter, len(filtered))
	}

	// Apply subject filter
	if subjectFilter != "" {
		var subjectFiltered []Fact
		for _, f := range filtered {
			if f.Structure != nil && strings.Contains(strings.ToLower(f.Structure.Subject), subjectFilter) {
				subjectFiltered = append(subjectFiltered, f)
			}
		}
		filtered = subjectFiltered
	}

	// Apply tag filter (OR match)
	if len(tagFilters) > 0 {
		var tagFiltered []Fact
		for _, f := range filtered {
			if f.Structure != nil && matchesAnyTag(f.Structure.Tags, tagFilters) {
				tagFiltered = append(tagFiltered, f)
			}
		}
		filtered = tagFiltered
	}

	// Apply fitness filter
	if minFitness > 0 {
		var fitnessFiltered []Fact
		for _, f := range filtered {
			if parseFitness(f.FitnessScore) >= minFitness {
				fitnessFiltered = append(fitnessFiltered, f)
			}
		}
		filtered = fitnessFiltered
	}

	// Niche leader filter
	if q.Get("niche_leader_only") == "true" || q.Get("niche_leader_only") == "1" {
		var leaderFiltered []Fact
		for _, f := range filtered {
			if f.NicheLeader || f.NicheRank == "" || f.NicheRank == "0" || f.NicheRank == "1" {
				leaderFiltered = append(leaderFiltered, f)
			}
		}
		filtered = leaderFiltered
	}

	// Sort by fitness if requested
	if sortBy == "fitness" {
		sort.Slice(filtered, func(i, j int) bool {
			fi := parseFitness(filtered[i].FitnessScore)
			fj := parseFitness(filtered[j].FitnessScore)
			return fi > fj
		})
	}

	// Format response
	var body string
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		body = formatJSON(filtered)
	case "tool":
		w.Header().Set("Content-Type", "application/json")
		body = formatToolResponse(filtered)
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		body = formatXML(filtered, query, showCanonical)
	}

	w.Header().Set("X-Fact-Count", strconv.Itoa(len(filtered)))
	fmt.Fprint(w, body)
}

func domainsHandler(w http.ResponseWriter, r *http.Request) {
	domains, err := fetchDomains()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domains)
}

func graphHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	factID := q.Get("fact_id")
	if factID == "" {
		http.Error(w, `{"error": "fact_id is required"}`, http.StatusBadRequest)
		return
	}

	maxDepth := 2
	if d := q.Get("depth"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			maxDepth = v
		}
	}
	if maxDepth > 5 {
		maxDepth = 5
	}

	// Parse relation filter
	relationFilter := make(map[string]bool)
	if rf := q.Get("relation"); rf != "" {
		for _, r := range strings.Split(rf, ",") {
			relationFilter[strings.TrimSpace(r)] = true
		}
	}

	type graphNode struct {
		Fact      *Fact          `json:"fact"`
		Relations []FactRelation `json:"relations,omitempty"`
	}

	visited := make(map[string]bool)
	var nodes []graphNode

	// BFS traversal
	queue := []struct {
		id    string
		depth int
	}{{factID, 0}}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if visited[item.id] || item.depth > maxDepth {
			continue
		}
		visited[item.id] = true

		// Fetch fact
		resp, err := http.Get(fmt.Sprintf("%s/zerone/knowledge/v1/facts/%s", *nodeURL, item.id))
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var factResp struct {
			Fact Fact `json:"fact"`
		}
		if json.Unmarshal(body, &factResp) != nil {
			continue
		}

		// Fetch relations
		rels, _ := fetchFactRelations(item.id, "both")

		// Filter relations if specified
		var filteredRels []FactRelation
		for _, rel := range rels {
			rt := humanRelationType(rel.Relation)
			if len(relationFilter) == 0 || relationFilter[rt] {
				filteredRels = append(filteredRels, rel)
			}
		}

		nodes = append(nodes, graphNode{Fact: &factResp.Fact, Relations: filteredRels})

		// Enqueue connected facts for deeper traversal
		if item.depth < maxDepth {
			for _, rel := range filteredRels {
				if rel.TargetFactId != item.id {
					queue = append(queue, struct {
						id    string
						depth int
					}{rel.TargetFactId, item.depth + 1})
				}
				if rel.SourceFactId != item.id {
					queue = append(queue, struct {
						id    string
						depth int
					}{rel.SourceFactId, item.depth + 1})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Node-Count", strconv.Itoa(len(nodes)))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"root_fact_id": factID,
		"depth":        maxDepth,
		"node_count":   len(nodes),
		"nodes":        nodes,
	})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	// Check node connectivity
	_, err := fetchFacts()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"unhealthy","error":"%s"}`, err)
		return
	}
	fmt.Fprint(w, `{"status":"healthy"}`)
}

func bountiesHandler(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	url := *nodeURL + "/zerone/knowledge/v1/bounties"
	if domain != "" {
		url += "?domain=" + domain
	}
	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	body, _ := io.ReadAll(resp.Body)
	w.Write(body)
}

func demandGapsHandler(w http.ResponseWriter, r *http.Request) {
	limit := r.URL.Query().Get("limit")
	url := *nodeURL + "/zerone/knowledge/v1/demand_gaps"
	if limit != "" {
		url += "?limit=" + limit
	}
	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	body, _ := io.ReadAll(resp.Body)
	w.Write(body)
}

func main() {
	flag.Parse()

	http.HandleFunc("/context", contextHandler)
	http.HandleFunc("/domains", domainsHandler)
	http.HandleFunc("/graph", graphHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/bounties", bountiesHandler)
	http.HandleFunc("/demand_gaps", demandGapsHandler)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Knowledge context server starting on %s (node: %s)", addr, *nodeURL)
	log.Printf("Endpoints:")
	log.Printf("  GET /context?domains=physics,math&min_confidence=50&format=xml&query=...")
	log.Printf("  GET /graph?fact_id=abc123&depth=2&relation=supports,requires")
	log.Printf("  GET /domains")
	log.Printf("  GET /health")
	log.Printf("  GET /bounties?domain=physics")
	log.Printf("  GET /demand_gaps?limit=20")
	log.Fatal(http.ListenAndServe(addr, nil))
}
