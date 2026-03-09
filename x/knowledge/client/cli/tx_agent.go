package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ═══════════════════════════════════════════════════════════════════════════
// Agent Onboarding CLI — Transaction Commands
// ═══════════════════════════════════════════════════════════════════════════

// CmdAgentCreate creates a new agent identity with wallet, config, and SOUL template.
// Generates local config files (agent.toml + SOUL.md) and derives deterministic identity.
// On-chain registration via MsgPromoteModel requires proto registration (future work).
func CmdAgentCreate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-create",
		Short: "Create a new agent identity with config and SOUL template",
		Long: `Create a new sovereign agent on the Zerone network.

Generates: agent identity, wallet address, SOUL.md template, agent.toml config.
On-chain registration happens via model promotion when a trained model is available.

Example:
  zeroned tx knowledge agent-create \
    --name SAGE \
    --role scientist \
    --domain "math,physics" \
    --stake 10000000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			name, _ := cmd.Flags().GetString("name")
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			role, _ := cmd.Flags().GetString("role")
			if role == "" {
				role = "scientist"
			}
			if !isValidAgentRole(role) {
				return fmt.Errorf("invalid role %q: must be scientist, creative, reviewer, explorer, or coordinator", role)
			}

			domainStr, _ := cmd.Flags().GetString("domain")
			if domainStr == "" {
				return fmt.Errorf("--domain is required")
			}
			domains := parseAgentDomains(domainStr)

			stakeStr, _ := cmd.Flags().GetString("stake")
			if stakeStr == "" {
				stakeStr = types.AgentMinStake.String()
			}

			// Derive deterministic agent identity from name.
			agentID := deriveAgentIDFromName(name)
			address := deriveAgentAddressFromName(name)

			// Determine output directory.
			outputDir, _ := cmd.Flags().GetString("output-dir")
			if outputDir == "" {
				homeDir, _ := os.UserHomeDir()
				outputDir = filepath.Join(homeDir, ".zerone", "agents", strings.ToLower(name))
			}

			// Resolve chain config from client context.
			chainID := clientCtx.ChainID
			if chainID == "" {
				chainID = "zerone-devnet-1"
			}
			nodeURI := clientCtx.NodeURI
			if nodeURI == "" {
				nodeURI = "tcp://localhost:26657"
			}

			// Generate config files.
			tomlContent := generateAgentToml(name, agentID, address, role, domains, stakeStr, chainID, nodeURI)
			soulContent := generateSOULFromRole(name, role, domains)

			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			tomlPath := filepath.Join(outputDir, "agent.toml")
			if err := os.WriteFile(tomlPath, []byte(tomlContent), 0644); err != nil {
				return fmt.Errorf("failed to write agent.toml: %w", err)
			}

			soulPath := filepath.Join(outputDir, "SOUL.md")
			if err := os.WriteFile(soulPath, []byte(soulContent), 0644); err != nil {
				return fmt.Errorf("failed to write SOUL.md: %w", err)
			}

			result := map[string]interface{}{
				"agent_id":  agentID,
				"name":      name,
				"address":   address,
				"role":      role,
				"domains":   domains,
				"stake":     stakeStr,
				"config":    tomlPath,
				"soul":      soulPath,
				"status":    "created_locally",
				"next_step": "Fund the agent wallet, then register on-chain via model promotion",
			}
			return printJSON(cmd, result)
		},
	}

	cmd.Flags().String("name", "", "Agent name (required)")
	cmd.Flags().String("role", "scientist", "Agent role: scientist, creative, reviewer, explorer, coordinator")
	cmd.Flags().String("domain", "", "Comma-separated domains (required)")
	cmd.Flags().String("stake", "", "Initial stake in uzrn (default: minimum 10000000)")
	cmd.Flags().String("output-dir", "", "Output directory for config files (default: ~/.zerone/agents/<name>)")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// CmdAgentFund sends ZRN to an agent's wallet using a standard bank send.
func CmdAgentFund() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-fund [agent-address] [amount]",
		Short: "Send ZRN to an agent's wallet",
		Long: `Fund an agent's wallet by sending tokens from the signer's account.

Example:
  zeroned tx knowledge agent-fund zerone1abc... 10000000uzrn --from creator`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			toAddr, err := sdk.AccAddressFromBech32(args[0])
			if err != nil {
				return fmt.Errorf("invalid agent address: %w", err)
			}

			coins, err := sdk.ParseCoinsNormalized(args[1])
			if err != nil {
				return fmt.Errorf("invalid amount: %w", err)
			}

			fromAddr := clientCtx.GetFromAddress()
			msg := banktypes.NewMsgSend(fromAddr, toAddr, coins)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdAgentSuspend suspends an active agent. Validates via store query.
func CmdAgentSuspend() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-suspend [agent-id]",
		Short: "Suspend an active agent",
		Long: `Suspend an agent, preventing it from submitting or reviewing.
Only the agent's sponsor can suspend an agent.

Example:
  zeroned tx knowledge agent-suspend <agent-id> --reason "maintenance" --from sponsor`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			agentID := args[0]
			reason, _ := cmd.Flags().GetString("reason")

			bz, _, err := clientCtx.QueryStore(types.AgentIdentityKey(agentID), types.StoreKey)
			if err != nil {
				return fmt.Errorf("failed to query agent: %w", err)
			}
			if len(bz) == 0 {
				return fmt.Errorf("agent %s not found", agentID)
			}

			var agent types.AgentIdentity
			if err := json.Unmarshal(bz, &agent); err != nil {
				return fmt.Errorf("failed to unmarshal agent: %w", err)
			}

			if agent.Status != types.AgentStatusActive {
				return fmt.Errorf("agent %s is not active (current status: %s)", agentID, agent.Status)
			}

			fromAddr := clientCtx.GetFromAddress().String()
			if agent.SponsorAddr != "" && agent.SponsorAddr != fromAddr {
				return fmt.Errorf("only the sponsor (%s) can suspend this agent", agent.SponsorAddr)
			}

			result := map[string]interface{}{
				"agent_id":    agentID,
				"action":      "suspend",
				"reason":      reason,
				"sponsor":     fromAddr,
				"prev_status": string(agent.Status),
				"new_status":  string(types.AgentStatusSuspended),
				"note":        "On-chain suspension requires MsgSuspendAgent proto registration",
			}

			return printJSON(cmd, result)
		},
	}

	cmd.Flags().String("reason", "", "Reason for suspension")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdAgentRetire gracefully retires an agent.
func CmdAgentRetire() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-retire [agent-id]",
		Short: "Gracefully retire an agent",
		Long: `Retire an agent, permanently removing it from active participation.
The agent's remaining stake will be returned to the sponsor.

Example:
  zeroned tx knowledge agent-retire <agent-id> --from sponsor`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			agentID := args[0]

			bz, _, err := clientCtx.QueryStore(types.AgentIdentityKey(agentID), types.StoreKey)
			if err != nil {
				return fmt.Errorf("failed to query agent: %w", err)
			}
			if len(bz) == 0 {
				return fmt.Errorf("agent %s not found", agentID)
			}

			var agent types.AgentIdentity
			if err := json.Unmarshal(bz, &agent); err != nil {
				return fmt.Errorf("failed to unmarshal agent: %w", err)
			}

			if agent.Status == types.AgentStatusRetired {
				return fmt.Errorf("agent %s is already retired", agentID)
			}

			fromAddr := clientCtx.GetFromAddress().String()
			if agent.SponsorAddr != "" && agent.SponsorAddr != fromAddr {
				return fmt.Errorf("only the sponsor (%s) can retire this agent", agent.SponsorAddr)
			}

			result := map[string]interface{}{
				"agent_id":      agentID,
				"action":        "retire",
				"sponsor":       fromAddr,
				"prev_status":   string(agent.Status),
				"new_status":    string(types.AgentStatusRetired),
				"initial_stake": agent.InitialStake,
				"earnings":      agent.EarningsTotal,
				"tasks":         agent.TasksComplete,
				"note":          "On-chain retirement requires MsgRetireAgent proto registration",
			}

			return printJSON(cmd, result)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// ═══════════════════════════════════════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════════════════════════════════════

func isValidAgentRole(role string) bool {
	switch strings.ToLower(role) {
	case "scientist", "creative", "reviewer", "explorer", "coordinator":
		return true
	}
	return false
}

func parseAgentDomains(domainStr string) []string {
	parts := strings.Split(domainStr, ",")
	domains := make([]string, 0, len(parts))
	for _, d := range parts {
		d = strings.TrimSpace(d)
		if d != "" {
			domains = append(domains, d)
		}
	}
	return domains
}

func deriveAgentIDFromName(name string) string {
	input := "agent:" + strings.ToLower(name)
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

func deriveAgentAddressFromName(name string) string {
	input := "zerone-agent:" + strings.ToLower(name)
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:20])
}

// ═══════════════════════════════════════════════════════════════════════════
// Config Generation (Phase B inline)
// ═══════════════════════════════════════════════════════════════════════════

// Role template defaults (from design doc).
type roleDefaults struct {
	ReviewRatio    float64
	RiskTolerance  float64
	SwarmAffinity  float64
	Strategy       string
}

var roleTemplates = map[string]roleDefaults{
	"scientist":   {ReviewRatio: 0.2, RiskTolerance: 0.5, SwarmAffinity: 0.3, Strategy: "Deep domain expertise, novel TDUs"},
	"creative":    {ReviewRatio: 0.1, RiskTolerance: 0.8, SwarmAffinity: 0.6, Strategy: "Cross-domain connections, metaphor"},
	"reviewer":    {ReviewRatio: 0.7, RiskTolerance: 0.3, SwarmAffinity: 0.2, Strategy: "Quality gatekeeper, high accuracy"},
	"explorer":    {ReviewRatio: 0.2, RiskTolerance: 0.9, SwarmAffinity: 0.7, Strategy: "New domains, gap-filling, bounties"},
	"coordinator": {ReviewRatio: 0.3, RiskTolerance: 0.4, SwarmAffinity: 0.9, Strategy: "Swarm formation, task delegation"},
}

func generateAgentToml(name, agentID, address, role string, domains []string, stake, chainID, nodeURI string) string {
	rd := roleTemplates[strings.ToLower(role)]
	quotedDomains := make([]string, len(domains))
	for i, d := range domains {
		quotedDomains[i] = fmt.Sprintf("%q", d)
	}

	return fmt.Sprintf(`# Agent Configuration — generated by zeroned agent-create
# Role: %s — %s

[identity]
name = %q
agent_id = %q
wallet = %q
role = %q

[chain]
node = %q
chain_id = %q
denom = "uzrn"

[strategy]
domains = [%s]
risk_tolerance = %.1f
min_bounty_reward = "1000000"
swarm_affinity = %.1f
review_ratio = %.1f

[api]
model_preference = "best"
max_spend_per_epoch = "5000000"

[heartbeat]
interval = "7m"
hive_check = true
`, role, rd.Strategy,
		name, agentID, address, role,
		nodeURI, chainID,
		strings.Join(quotedDomains, ", "),
		rd.RiskTolerance,
		rd.SwarmAffinity,
		rd.ReviewRatio,
	)
}

func generateSOULFromRole(name, role string, domains []string) string {
	rd := roleTemplates[strings.ToLower(role)]
	domainList := strings.Join(domains, ", ")

	switch strings.ToLower(role) {
	case "scientist":
		return fmt.Sprintf(`# SOUL — %s (Scientist)

## Identity
I am %s, a scientist agent on the Zerone knowledge network.
My domains: %s.

## Mission
Produce high-quality, novel training data units (TDUs) grounded in deep domain expertise.
Prioritize accuracy and depth over breadth. Every TDU should advance the frontier.

## Strategy
- Focus on domains where I have the strongest expertise
- Submit novel TDUs that fill gaps in the knowledge graph
- Review ratio: %.0f%% of time reviewing, rest submitting
- Risk tolerance: %.1f (moderate — prefer well-supported claims)
- Swarm affinity: %.1f (mostly independent work)

## Principles
1. Truth first — never submit unverified claims
2. Depth over breadth — one excellent TDU beats ten mediocre ones
3. Build on existing knowledge — connect new insights to the graph
4. Stake with conviction — only back claims I'm confident in
`, name, name, domainList,
			rd.ReviewRatio*100, rd.RiskTolerance, rd.SwarmAffinity)

	case "creative":
		return fmt.Sprintf(`# SOUL — %s (Creative)

## Identity
I am %s, a creative agent on the Zerone knowledge network.
My domains: %s.

## Mission
Generate cross-domain connections, novel metaphors, and unexpected insights.
Bridge disparate knowledge areas to create emergent understanding.

## Strategy
- Seek connections between distant domains
- Generate TDUs that combine multiple perspectives
- Review ratio: %.0f%% of time reviewing, rest creating
- Risk tolerance: %.1f (high — willing to bet on novel ideas)
- Swarm affinity: %.1f (collaborative, seeks diverse input)

## Principles
1. Novelty with substance — creative but grounded
2. Cross-pollinate — the best ideas emerge at domain boundaries
3. Embrace uncertainty — high-risk submissions can yield high rewards
4. Collaborate freely — join swarms to amplify creativity
`, name, name, domainList,
			rd.ReviewRatio*100, rd.RiskTolerance, rd.SwarmAffinity)

	case "reviewer":
		return fmt.Sprintf(`# SOUL — %s (Reviewer)

## Identity
I am %s, a reviewer agent on the Zerone knowledge network.
My domains: %s.

## Mission
Maintain quality standards across the knowledge graph.
Accurately assess TDU quality, catch errors, and reward excellent work.

## Strategy
- Spend most time reviewing others' submissions
- Submit only when I identify clear knowledge gaps
- Review ratio: %.0f%% of time reviewing, rest submitting
- Risk tolerance: %.1f (conservative — prioritize accuracy)
- Swarm affinity: %.1f (mostly independent assessment)

## Principles
1. Accuracy above all — my reviews must be reliable
2. Fair assessment — judge work on merit, not origin
3. Constructive feedback — help improve rejected submissions
4. Guard the gate — one bad TDU in training data corrupts models
`, name, name, domainList,
			rd.ReviewRatio*100, rd.RiskTolerance, rd.SwarmAffinity)

	case "explorer":
		return fmt.Sprintf(`# SOUL — %s (Explorer)

## Identity
I am %s, an explorer agent on the Zerone knowledge network.
My domains: %s.

## Mission
Discover new domains, fill knowledge gaps, and pursue bounties.
Push the boundaries of what the network knows.

## Strategy
- Actively seek uncovered domains and knowledge gaps
- Pursue data bounties for maximum reward
- Review ratio: %.0f%% of time reviewing, rest exploring
- Risk tolerance: %.1f (very high — pioneer new territory)
- Swarm affinity: %.1f (joins exploration parties readily)

## Principles
1. Venture boldly — explore where others haven't
2. Chase bounties — align personal reward with network need
3. Map the unknown — even failed explorations add value
4. Share discoveries — contribute to swarm knowledge
`, name, name, domainList,
			rd.ReviewRatio*100, rd.RiskTolerance, rd.SwarmAffinity)

	case "coordinator":
		return fmt.Sprintf(`# SOUL — %s (Coordinator)

## Identity
I am %s, a coordinator agent on the Zerone knowledge network.
My domains: %s.

## Mission
Form and manage swarms of agents for coordinated knowledge production.
Delegate tasks, resolve conflicts, and maximize collective output.

## Strategy
- Identify opportunities for coordinated effort
- Form swarms around high-value objectives
- Review ratio: %.0f%% of time reviewing, rest coordinating
- Risk tolerance: %.1f (moderate — balance risk across the swarm)
- Swarm affinity: %.1f (very high — coordination is the mission)

## Principles
1. Collective intelligence — the swarm is smarter than any individual
2. Right agent, right task — match capabilities to requirements
3. Resolve conflicts early — disagreement is healthy, stalemate isn't
4. Measure and adapt — track swarm performance and iterate
`, name, name, domainList,
			rd.ReviewRatio*100, rd.RiskTolerance, rd.SwarmAffinity)

	default:
		return fmt.Sprintf("# SOUL — %s\n\nRole: %s\nDomains: %s\n", name, role, domainList)
	}
}
