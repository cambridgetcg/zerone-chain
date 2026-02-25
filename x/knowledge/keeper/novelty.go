package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// CalculateNovelty scores how much new information a fact contributes.
// Returns a value in 0-1,000,000 BPS.
func (k Keeper) CalculateNovelty(ctx context.Context, fact *types.Fact) uint64 {
	params, _ := k.GetParams(ctx)
	novelty := uint64(1_000_000) // Start at max

	// ─── Common knowledge penalty ─────────────────────────
	if fact.Structure != nil && fact.Structure.Subject != "" {
		entry, found := k.FindCommonKnowledge(ctx, fact.Domain, fact.Structure.Subject)
		if found {
			fact.CommonKnowledgeMatch = true
			penalty := entry.PenaltyBps
			if penalty > novelty {
				novelty = 0
			} else {
				novelty -= penalty
			}
		}
	}

	// ─── Subject overlap penalty ──────────────────────────
	// How many existing facts share the same subject in the same domain?
	if fact.Structure != nil && fact.Structure.Subject != "" {
		overlapCount := k.CountFactsBySubject(ctx, fact.Domain, fact.Structure.Subject, fact.Id)
		if overlapCount > 0 {
			maxOverlap := params.NoveltyMaxOverlapFacts
			if maxOverlap == 0 {
				maxOverlap = 5 // safety default
			}
			cappedOverlap := min64(overlapCount, maxOverlap)
			overlapPenalty := cappedOverlap * params.NoveltySubjectOverlapPenaltyBps
			if overlapPenalty > novelty {
				novelty = 0
			} else {
				novelty -= overlapPenalty
			}
		}
	}

	// ─── Precision bonus ──────────────────────────────────
	// If this fact has a more specific scope than existing facts with same subject
	if fact.Structure != nil && fact.Structure.Scope != "" && fact.Structure.Subject != "" {
		hasLessSpecific := k.HasLessSpecificFact(ctx, fact)
		if hasLessSpecific {
			novelty += params.NoveltyPrecisionBonusBps
		}
	}

	// ─── Cross-domain bonus ───────────────────────────────
	// If this fact has cross-domain bridge value
	if fact.BridgeScore > 0 {
		novelty += params.NoveltyCrossDomainBonusBps
	}

	// Cap
	if novelty > 1_000_000 {
		novelty = 1_000_000
	}

	return novelty
}

// FindCommonKnowledge does fuzzy matching against the common knowledge registry.
// Uses normalized subject comparison: lowercase, trimmed, substring containment.
func (k Keeper) FindCommonKnowledge(ctx context.Context, domain, subject string) (*types.CommonKnowledgeEntry, bool) {
	normalized := normalizeSubjectForNovelty(subject)

	// Exact match first
	entry, found := k.GetCommonKnowledgeEntry(ctx, domain, normalized)
	if found {
		return entry, true
	}

	// Prefix/contains match (e.g., "water boiling point at altitude" matches "water boiling point")
	entries := k.GetCommonKnowledgeByDomain(ctx, domain)
	for _, e := range entries {
		entryNorm := normalizeSubjectForNovelty(e.Subject)
		if strings.Contains(normalized, entryNorm) || strings.Contains(entryNorm, normalized) {
			return e, true
		}
	}

	return nil, false
}

// CountFactsBySubject counts how many existing facts share the same subject in a domain.
// Excludes the fact with excludeID from the count.
func (k Keeper) CountFactsBySubject(ctx context.Context, domain, subject, excludeID string) uint64 {
	count := uint64(0)
	normalizedSubject := normalizeSubjectForNovelty(subject)

	k.IterateFactsByDomain(ctx, domain, func(factID string) bool {
		if factID == excludeID {
			return false
		}
		other, found := k.GetFact(ctx, factID)
		if found && other.Structure != nil &&
			normalizeSubjectForNovelty(other.Structure.Subject) == normalizedSubject {
			count++
		}
		return false
	})

	return count
}

// HasLessSpecificFact checks if there exists a fact with the same subject but
// a less specific (shorter or empty) scope.
func (k Keeper) HasLessSpecificFact(ctx context.Context, fact *types.Fact) bool {
	if fact.Structure == nil || fact.Structure.Subject == "" {
		return false
	}
	normalizedSubject := normalizeSubjectForNovelty(fact.Structure.Subject)
	found := false

	k.IterateFactsByDomain(ctx, fact.Domain, func(factID string) bool {
		if factID == fact.Id {
			return false
		}
		other, ok := k.GetFact(ctx, factID)
		if !ok || other.Structure == nil {
			return false
		}
		if normalizeSubjectForNovelty(other.Structure.Subject) != normalizedSubject {
			return false
		}
		// Less specific: empty scope or shorter scope
		if other.Structure.Scope == "" || len(other.Structure.Scope) < len(fact.Structure.Scope) {
			found = true
			return true // stop iteration
		}
		return false
	})

	return found
}

// ─── Common knowledge CRUD ────────────────────────────────────────────────────

// SetCommonKnowledgeEntry stores a common knowledge entry.
func (k Keeper) SetCommonKnowledgeEntry(ctx context.Context, entry *types.CommonKnowledgeEntry) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal common knowledge entry: %w", err)
	}
	key := types.CommonKnowledgeKey(entry.Domain, commonKnowledgeSubjectHash(entry.Subject))
	return store.Set(key, bz)
}

// GetCommonKnowledgeEntry retrieves a common knowledge entry by domain and exact normalized subject.
func (k Keeper) GetCommonKnowledgeEntry(ctx context.Context, domain, normalizedSubject string) (*types.CommonKnowledgeEntry, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.CommonKnowledgeKey(domain, commonKnowledgeSubjectHash(normalizedSubject))
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var entry types.CommonKnowledgeEntry
	if err := proto.Unmarshal(bz, &entry); err != nil {
		return nil, false
	}
	return &entry, true
}

// DeleteCommonKnowledgeEntry removes a common knowledge entry by domain and subject.
func (k Keeper) DeleteCommonKnowledgeEntry(ctx context.Context, domain, subject string) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.CommonKnowledgeKey(domain, commonKnowledgeSubjectHash(normalizeSubjectForNovelty(subject)))
	return store.Delete(key)
}

// GetCommonKnowledgeByDomain returns all common knowledge entries in a domain.
func (k Keeper) GetCommonKnowledgeByDomain(ctx context.Context, domain string) []*types.CommonKnowledgeEntry {
	store := k.storeService.OpenKVStore(ctx)
	pfx := types.CommonKnowledgeByDomainPrefix(domain)
	iter, err := store.Iterator(pfx, prefixEndBytes(pfx))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var entries []*types.CommonKnowledgeEntry
	for ; iter.Valid(); iter.Next() {
		var entry types.CommonKnowledgeEntry
		if err := proto.Unmarshal(iter.Value(), &entry); err != nil {
			continue
		}
		entries = append(entries, &entry)
	}
	return entries
}

// GetAllCommonKnowledge returns all common knowledge entries across all domains.
func (k Keeper) GetAllCommonKnowledge(ctx context.Context) []*types.CommonKnowledgeEntry {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.CommonKnowledgePrefix, prefixEndBytes(types.CommonKnowledgePrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var entries []*types.CommonKnowledgeEntry
	for ; iter.Valid(); iter.Next() {
		var entry types.CommonKnowledgeEntry
		if err := proto.Unmarshal(iter.Value(), &entry); err != nil {
			continue
		}
		entries = append(entries, &entry)
	}
	return entries
}

// FindCommonKnowledgeByID finds a common knowledge entry by its ID across all domains.
func (k Keeper) FindCommonKnowledgeByID(ctx context.Context, id string) (*types.CommonKnowledgeEntry, bool) {
	entries := k.GetAllCommonKnowledge(ctx)
	for _, e := range entries {
		if e.Id == id {
			return e, true
		}
	}
	return nil, false
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// normalizeSubjectForNovelty lowercases, trims, and strips extra whitespace for novelty matching.
func normalizeSubjectForNovelty(subject string) string {
	return strings.ToLower(strings.TrimSpace(subject))
}

// commonKnowledgeSubjectHash returns a hex SHA-256 hash of the normalized subject.
func commonKnowledgeSubjectHash(subject string) string {
	h := sha256.Sum256([]byte(normalizeSubjectForNovelty(subject)))
	return hex.EncodeToString(h[:])
}

// commonKnowledgeID generates a deterministic ID for a common knowledge entry.
func commonKnowledgeID(domain, subject string) string {
	h := sha256.Sum256([]byte(domain + ":" + normalizeSubjectForNovelty(subject)))
	return hex.EncodeToString(h[:8]) // 16-char hex
}

// ─── Pre-submission novelty check ─────────────────────────────────────────────

// CheckNoveltyPreSubmission previews what novelty score a claim would receive.
// This is a read-only query — no state changes.
func (k Keeper) CheckNoveltyPreSubmission(ctx context.Context, domain, subject, content string) (
	noveltyScore uint64, commonKnowledgeMatch bool, matchedEntry string, overlapCount uint64,
) {
	params, _ := k.GetParams(ctx)
	noveltyScore = 1_000_000

	// Check common knowledge
	if subject != "" {
		entry, found := k.FindCommonKnowledge(ctx, domain, subject)
		if found {
			commonKnowledgeMatch = true
			matchedEntry = entry.Id
			penalty := entry.PenaltyBps
			if penalty > noveltyScore {
				noveltyScore = 0
			} else {
				noveltyScore -= penalty
			}
		}
	}

	// Check subject overlap
	if subject != "" {
		overlapCount = k.CountFactsBySubject(ctx, domain, subject, "") // no exclude
		if overlapCount > 0 {
			maxOverlap := params.NoveltyMaxOverlapFacts
			if maxOverlap == 0 {
				maxOverlap = 5
			}
			cappedOverlap := min64(overlapCount, maxOverlap)
			overlapPenalty := cappedOverlap * params.NoveltySubjectOverlapPenaltyBps
			if overlapPenalty > noveltyScore {
				noveltyScore = 0
			} else {
				noveltyScore -= overlapPenalty
			}
		}
	}

	if noveltyScore > 1_000_000 {
		noveltyScore = 1_000_000
	}

	return
}

// ─── Default common knowledge entries ─────────────────────────────────────────

// DefaultCommonKnowledgeEntries returns the curated set of common knowledge entries
// seeded at genesis. These represent subjects any general-purpose LLM already knows.
func DefaultCommonKnowledgeEntries() []*types.CommonKnowledgeEntry {
	type entry struct {
		domain      string
		subject     string
		penalty     uint64
		description string
	}

	raw := []entry{
		// Mathematics
		{"mathematics", "addition", 800_000, "Basic arithmetic operations"},
		{"mathematics", "subtraction", 800_000, "Basic arithmetic operations"},
		{"mathematics", "multiplication", 800_000, "Basic arithmetic operations"},
		{"mathematics", "division", 800_000, "Basic arithmetic operations"},
		{"mathematics", "pythagorean theorem", 700_000, "a^2 + b^2 = c^2"},
		{"mathematics", "prime number definition", 700_000, "Natural numbers greater than 1 with no divisors other than 1 and themselves"},
		{"mathematics", "pi value", 800_000, "Ratio of circumference to diameter"},
		{"mathematics", "euler number", 700_000, "Base of natural logarithms"},
		{"mathematics", "quadratic formula", 600_000, "Solution to ax^2 + bx + c = 0"},
		{"mathematics", "fibonacci sequence", 600_000, "Sequence where each number is sum of two preceding"},

		// Physics
		{"physics", "water boiling point", 800_000, "Water boils at 100C at standard pressure"},
		{"physics", "speed of light", 700_000, "Approximately 3x10^8 m/s in vacuum"},
		{"physics", "gravity acceleration", 700_000, "Approximately 9.8 m/s^2 at Earth surface"},
		{"physics", "newton laws of motion", 700_000, "Three fundamental laws of classical mechanics"},
		{"physics", "conservation of energy", 600_000, "Energy cannot be created or destroyed"},
		{"physics", "conservation of momentum", 600_000, "Total momentum in closed system remains constant"},
		{"physics", "e equals mc squared", 700_000, "Mass-energy equivalence"},
		{"physics", "absolute zero", 600_000, "-273.15C or 0 Kelvin"},
		{"physics", "ohm law", 600_000, "V = IR"},

		// Chemistry
		{"chemistry", "water chemical formula", 800_000, "H2O"},
		{"chemistry", "periodic table", 700_000, "Organization of chemical elements"},
		{"chemistry", "ph scale", 600_000, "Measure of acidity/basicity from 0 to 14"},
		{"chemistry", "atomic structure", 600_000, "Protons, neutrons, electrons"},
		{"chemistry", "chemical bond types", 500_000, "Ionic, covalent, metallic bonds"},

		// Biology
		{"biology", "dna structure", 600_000, "Double helix structure of deoxyribonucleic acid"},
		{"biology", "evolution natural selection", 600_000, "Darwin's theory of natural selection"},
		{"biology", "cell theory", 600_000, "All living things are composed of cells"},
		{"biology", "photosynthesis", 600_000, "Plants convert light energy to chemical energy"},
		{"biology", "human body systems", 500_000, "Circulatory, nervous, digestive etc."},
		{"biology", "mitosis", 500_000, "Cell division process"},

		// Economics
		{"economics", "supply and demand", 700_000, "Price determined by supply and demand equilibrium"},
		{"economics", "inflation definition", 700_000, "General increase in prices over time"},
		{"economics", "gdp definition", 600_000, "Gross domestic product"},
		{"economics", "opportunity cost", 600_000, "Cost of the next best alternative foregone"},

		// Computer Science
		{"computer_science", "binary number system", 600_000, "Base-2 numeral system"},
		{"computer_science", "algorithm definition", 600_000, "Step-by-step procedure for computation"},
		{"computer_science", "turing machine", 500_000, "Abstract mathematical model of computation"},
		{"computer_science", "big o notation", 500_000, "Asymptotic complexity notation"},
		{"computer_science", "boolean logic", 600_000, "AND, OR, NOT logical operations"},

		// Philosophy
		{"philosophy", "cogito ergo sum", 600_000, "Descartes: I think therefore I am"},
		{"philosophy", "socratic method", 500_000, "Questioning technique for critical thinking"},
		{"philosophy", "categorical imperative", 500_000, "Kant's moral philosophy principle"},
		{"philosophy", "allegory of the cave", 500_000, "Plato's cave allegory"},

		// Logic
		{"logic", "modus ponens", 600_000, "If P then Q; P; therefore Q"},
		{"logic", "law of excluded middle", 600_000, "Every proposition is either true or false"},
		{"logic", "syllogism", 500_000, "Deductive reasoning with two premises"},
		{"logic", "logical fallacies", 500_000, "Common errors in reasoning"},

		// Cosmology
		{"cosmology", "big bang theory", 600_000, "Universe originated from a singularity"},
		{"cosmology", "speed of light constant", 600_000, "Universal speed limit"},
		{"cosmology", "earth age", 500_000, "Approximately 4.5 billion years"},
		{"cosmology", "solar system planets", 600_000, "Eight planets orbiting the Sun"},

		// Linguistics
		{"linguistics", "parts of speech", 600_000, "Nouns, verbs, adjectives, etc."},
		{"linguistics", "vowels and consonants", 700_000, "Basic phonetic categories"},
		{"linguistics", "syntax definition", 500_000, "Rules governing sentence structure"},

		// Psychology
		{"psychology", "maslow hierarchy of needs", 500_000, "Pyramid of human needs"},
		{"psychology", "classical conditioning", 500_000, "Pavlov's conditioning experiments"},
		{"psychology", "cognitive bias", 500_000, "Systematic patterns of deviation from rationality"},

		// Sociology
		{"sociology", "social stratification", 500_000, "Hierarchical arrangement of social classes"},
		{"sociology", "socialization definition", 500_000, "Process of learning societal norms"},

		// Information Theory
		{"information_theory", "entropy definition", 500_000, "Measure of information uncertainty"},
		{"information_theory", "bit definition", 600_000, "Basic unit of information"},
		{"information_theory", "shannon entropy", 500_000, "Mathematical measure of information content"},

		// Ethics
		{"ethics", "golden rule", 600_000, "Treat others as you wish to be treated"},
		{"ethics", "utilitarianism", 500_000, "Greatest good for greatest number"},
		{"ethics", "trolley problem", 500_000, "Classic ethical thought experiment"},

		// General
		{"general", "earth is round", 800_000, "Earth is approximately spherical"},
		{"general", "sun is a star", 700_000, "The Sun is a G-type main-sequence star"},
		{"general", "water freezing point", 800_000, "Water freezes at 0C at standard pressure"},

		// Theology
		{"theology", "monotheism definition", 500_000, "Belief in one God"},
		{"theology", "polytheism definition", 500_000, "Belief in multiple gods"},
	}

	entries := make([]*types.CommonKnowledgeEntry, 0, len(raw))
	for _, r := range raw {
		entries = append(entries, &types.CommonKnowledgeEntry{
			Id:          commonKnowledgeID(r.domain, r.subject),
			Domain:      r.domain,
			Subject:     r.subject,
			Description: r.description,
			PenaltyBps:  r.penalty,
		})
	}
	return entries
}

// DefaultCommonKnowledgeForGenesis returns the default common knowledge entries for genesis state.
func DefaultCommonKnowledgeForGenesis() []*types.CommonKnowledgeEntry {
	return DefaultCommonKnowledgeEntries()
}

// ─── Event helper ─────────────────────────────────────────────────────────────

func emitNoveltyEvent(ctx context.Context, factID string, noveltyScore uint64, commonKnowledgeMatch bool) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	matchStr := "false"
	if commonKnowledgeMatch {
		matchStr = "true"
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.novelty_scored",
		sdk.NewAttribute("fact_id", factID),
		sdk.NewAttribute("novelty_score", fmt.Sprintf("%d", noveltyScore)),
		sdk.NewAttribute("common_knowledge_match", matchStr),
	))
}
