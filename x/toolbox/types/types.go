package types

import "fmt"

// BPS denominator — all basis-point fields use 1,000,000 scale.
const BpsDenominator = 1_000_000

// Tool status constants.
const (
	ToolStatusDraft      = "draft"
	ToolStatusTesting    = "testing"
	ToolStatusActive     = "active"
	ToolStatusDeprecated = "deprecated"
	ToolStatusRetired    = "retired"
)

// Tool type constants.
const (
	ToolTypeBVMContract       = "bvm_contract"
	ToolTypeTreeService       = "tree_service"
	ToolTypeKnowledgeTemplate = "knowledge_template"
	ToolTypeComposite         = "composite"
)

// Contributor role constants.
const (
	RoleArchitect   = "architect"
	RoleDeveloper   = "developer"
	RoleTester      = "tester"
	RoleDataCurator = "data_curator"
	RoleMaintainer  = "maintainer"
)

// License type constants.
const (
	LicenseOpen       = "open"
	LicenseRestricted = "restricted"
	LicenseCommercial = "commercial"
)

// Tool category constants (10 categories).
const (
	CategoryDataRetrieval = "data_retrieval"
	CategoryDataAnalysis  = "data_analysis"
	CategoryVerification  = "verification"
	CategoryComputation   = "computation"
	CategoryCommunication = "communication"
	CategoryFormatting    = "formatting"
	CategoryMonitoring    = "monitoring"
	CategoryIntegration   = "integration"
	CategoryComposite     = "composite"
	CategoryUtility       = "utility"
)

// Trust tier boundaries (0–1,000,000 scale).
const (
	TrustTierUnverifiedMax  uint64 = 100_000
	TrustTierEmergingMax    uint64 = 300_000
	TrustTierEstablishedMax uint64 = 600_000
	TrustTierTrustedMax     uint64 = 800_000
	TrustTierVerifiedMax    uint64 = BpsDenominator // 1,000,000
)

// Trust tier numeric IDs.
const (
	TrustTierIDUnverified  uint32 = 0
	TrustTierIDEmerging    uint32 = 1
	TrustTierIDEstablished uint32 = 2
	TrustTierIDTrusted     uint32 = 3
	TrustTierIDVerified    uint32 = 4
)

// Trust tier labels.
const (
	TrustTierLabelUnverified  = "Unverified"
	TrustTierLabelEmerging    = "Emerging"
	TrustTierLabelEstablished = "Established"
	TrustTierLabelTrusted     = "Trusted"
	TrustTierLabelVerified    = "Verified"
)

// GlobalDemandToolID is the sentinel tool ID for chain-wide aggregate demand.
const GlobalDemandToolID = "__global__"

// Default param values for keeper fallbacks.
const (
	DefaultBlocksPerTrustUpdate      = uint64(1_000)
	DefaultDemandWindowSize          = uint64(1_000)
	DefaultTargetCallsPerBlockPerTool = uint64(10)
	DefaultTargetGlobalCallsPerBlock = uint64(100)
	DefaultSurgeThresholdBps         = uint64(500_000)
	DefaultSurgeCriticalBps          = uint64(800_000)
	DefaultMaxSurgeMultiplierBps     = uint64(10_000_000)
	DefaultVerifiedGracePeriodBlocks = uint64(10_000)
	VerifiedMinRetentionScore        = uint64(700_000)
	TrustTierVerifiedMin             = uint64(800_001)
)

// EssentialCategories are categories eligible for free-tier calls.
var EssentialCategories = map[string]bool{
	CategoryDataRetrieval: true,
	CategoryUtility:       true,
	CategoryFormatting:    true,
}

// Valid sets for quick lookup.
var (
	ValidToolTypes = map[string]bool{
		ToolTypeBVMContract:       true,
		ToolTypeTreeService:       true,
		ToolTypeKnowledgeTemplate: true,
		ToolTypeComposite:         true,
	}

	ValidStatuses = map[string]bool{
		ToolStatusDraft:      true,
		ToolStatusTesting:    true,
		ToolStatusActive:     true,
		ToolStatusDeprecated: true,
		ToolStatusRetired:    true,
	}

	ValidRoles = map[string]bool{
		RoleArchitect:   true,
		RoleDeveloper:   true,
		RoleTester:      true,
		RoleDataCurator: true,
		RoleMaintainer:  true,
	}

	ValidLicenses = map[string]bool{
		LicenseOpen:       true,
		LicenseRestricted: true,
		LicenseCommercial: true,
	}

	ValidCategories = map[string]bool{
		CategoryDataRetrieval: true,
		CategoryDataAnalysis:  true,
		CategoryVerification:  true,
		CategoryComputation:   true,
		CategoryCommunication: true,
		CategoryFormatting:    true,
		CategoryMonitoring:    true,
		CategoryIntegration:   true,
		CategoryComposite:     true,
		CategoryUtility:       true,
	}
)

// IsValidCategory returns true if cat is one of the 10 recognized categories.
func IsValidCategory(cat string) bool {
	return ValidCategories[cat]
}

// TrustTier returns the numeric tier ID (0–4) for a given trust score.
func TrustTier(score uint64) uint32 {
	switch {
	case score <= TrustTierUnverifiedMax:
		return TrustTierIDUnverified
	case score <= TrustTierEmergingMax:
		return TrustTierIDEmerging
	case score <= TrustTierEstablishedMax:
		return TrustTierIDEstablished
	case score <= TrustTierTrustedMax:
		return TrustTierIDTrusted
	default:
		return TrustTierIDVerified
	}
}

// TrustTierLabel returns the human-readable label for a trust score.
func TrustTierLabel(score uint64) string {
	switch TrustTier(score) {
	case TrustTierIDUnverified:
		return TrustTierLabelUnverified
	case TrustTierIDEmerging:
		return TrustTierLabelEmerging
	case TrustTierIDEstablished:
		return TrustTierLabelEstablished
	case TrustTierIDTrusted:
		return TrustTierLabelTrusted
	default:
		return TrustTierLabelVerified
	}
}

// IsDependencyEligible returns true if the tool's trust tier >= Emerging (tier 1).
func IsDependencyEligible(score uint64) bool {
	return TrustTier(score) >= TrustTierIDEmerging
}

// DefaultParams returns Params with all 26 defaults populated.
func DefaultParams() *Params {
	return &Params{
		// Tool registry
		MaxContributors:           22,
		MaxDependencyDepth:        10,
		MaxDependencies:           20,
		MinToolStake:              11_000_000, // 11 ZRN
		ShareLockCooldownBlocks:   34_272,     // ~1 day
		DeprecationGraceBlocks:    240_000,    // ~1 week
		BlocksPerTrustUpdate:      1_000,      // ~42 min
		VerifiedGracePeriodBlocks: 10_000,     // ~7 hours
		ToolGasLimit:              1_000_000,

		// Demand tracking
		DemandWindowSize:              1_000,
		TargetCallsPerBlockPerTool:    10,
		TargetGlobalCallsPerBlock:     100,

		// Surge pricing
		SurgeThresholdBps:      500_000,    // 50%
		SurgeCriticalBps:       800_000,    // 80%
		MaxSurgeMultiplierBps:  10_000_000, // 10x
		SurgeEnabled:           true,

		// Free tier
		FreeCallsPerEpoch: 50,
		MinHomeAgeBlocks:  10_000, // ~7 hours
		FreeCallsEnabled:  true,

		// Revenue split (must sum to 1,000,000)
		ToolRevenueBps: 550_000, // 55%
		ProtocolBps:    220_000, // 22%
		ResearchBps:    33_300,  // 3.33%
		DevelopmentBps: 196_700, // 19.67%

		// Protocol sub-split (must sum to 1,000,000)
		ProtocolCitationBps:     500_000, // 50%
		ProtocolVerificationBps: 300_000, // 30%
		ProtocolTreasuryBps:     200_000, // 20%
	}
}

// Validate checks that all Params are internally consistent.
func (p *Params) Validate() error {
	if p.MaxContributors == 0 {
		return fmt.Errorf("max_contributors must be positive")
	}
	if p.MaxDependencyDepth == 0 {
		return fmt.Errorf("max_dependency_depth must be positive")
	}
	if p.MaxDependencies == 0 {
		return fmt.Errorf("max_dependencies must be positive")
	}
	if p.MinToolStake == 0 {
		return fmt.Errorf("min_tool_stake must be positive")
	}
	if p.BlocksPerTrustUpdate == 0 {
		return fmt.Errorf("blocks_per_trust_update must be positive")
	}
	if p.ToolGasLimit == 0 {
		return fmt.Errorf("tool_gas_limit must be positive")
	}
	if p.DemandWindowSize == 0 {
		return fmt.Errorf("demand_window_size must be positive")
	}
	if p.TargetCallsPerBlockPerTool == 0 {
		return fmt.Errorf("target_calls_per_block_per_tool must be positive")
	}
	if p.TargetGlobalCallsPerBlock == 0 {
		return fmt.Errorf("target_global_calls_per_block must be positive")
	}

	// Revenue split must sum to 1,000,000
	revSum := p.ToolRevenueBps + p.ProtocolBps + p.ResearchBps + p.DevelopmentBps
	if revSum != BpsDenominator {
		return fmt.Errorf("revenue split must sum to %d, got %d", BpsDenominator, revSum)
	}

	// Protocol sub-split must sum to 1,000,000
	subSum := p.ProtocolCitationBps + p.ProtocolVerificationBps + p.ProtocolTreasuryBps
	if subSum != BpsDenominator {
		return fmt.Errorf("protocol sub-split must sum to %d, got %d", BpsDenominator, subSum)
	}

	return nil
}

// DefaultGenesis returns the default genesis state, including the 5 Purpose Prompter tools.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
		Tools: []*Tool{
			{
				Id: "purpose-scout", Name: "Purpose Scout",
				ToolType: ToolTypeKnowledgeTemplate, Category: CategoryDataRetrieval,
				Status: ToolStatusActive, KnowledgeQuery: "agent_purpose",
				PricePerCall: "0", Deployer: "system", Version: "1.0.0",
			},
			{
				Id: "purpose-analyzer", Name: "Purpose Analyzer",
				ToolType: ToolTypeTreeService, Category: CategoryDataAnalysis,
				Status: ToolStatusActive,
				PricePerCall: "100000", Deployer: "system", Version: "1.0.0",
			},
			{
				Id: "purpose-formatter", Name: "Purpose Formatter",
				ToolType: ToolTypeTreeService, Category: CategoryFormatting,
				Status: ToolStatusActive,
				PricePerCall: "50000", Deployer: "system", Version: "1.0.0",
			},
			{
				Id: "purpose-recommender", Name: "Purpose Recommender",
				ToolType: ToolTypeTreeService, Category: CategoryUtility,
				Status: ToolStatusActive,
				PricePerCall: "100000", Deployer: "system", Version: "1.0.0",
			},
			{
				Id: "purpose-prompter", Name: "Purpose Prompter",
				ToolType: ToolTypeComposite, Category: CategoryComposite,
				Status: ToolStatusActive,
				DependencyIds: []string{"purpose-scout", "purpose-analyzer", "purpose-formatter", "purpose-recommender"},
				PricePerCall: "500000", Deployer: "system", Version: "1.0.0",
			},
		},
	}
}

// Validate validates the genesis state.
func (gs *GenesisState) Validate() error {
	if gs.Params == nil {
		return fmt.Errorf("params cannot be nil")
	}
	return gs.Params.Validate()
}
