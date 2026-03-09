package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ═══════════════════════════════════════════════════════════════════════════
// Agent Onboarding CLI — Query Commands
// ═══════════════════════════════════════════════════════════════════════════

// CmdAgentList lists registered agents on the network.
// Uses ABCI subspace query to iterate the agent identity store prefix.
func CmdAgentList() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-list",
		Short: "List registered agents on the network",
		Long: `List all registered agents, optionally filtered by domain or status.

Example:
  zeroned q knowledge agent-list
  zeroned q knowledge agent-list --domain math
  zeroned q knowledge agent-list --status active`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			domain, _ := cmd.Flags().GetString("domain")
			statusFilter, _ := cmd.Flags().GetString("status")

			agents, err := queryAgentsBySubspace(clientCtx)
			if err != nil {
				// Fallback: report what we know and guide the user.
				result := map[string]interface{}{
					"error":   err.Error(),
					"message": "Use 'zeroned q knowledge agent-status <agent-id>' to query a specific agent",
				}
				return printJSON(cmd, result)
			}

			// Apply filters.
			var filtered []agentListEntry
			for _, a := range agents {
				if domain != "" && a.Domain != domain {
					continue
				}
				if statusFilter != "" && string(a.Status) != strings.ToLower(statusFilter) {
					continue
				}
				filtered = append(filtered, agentListEntry{
					AgentID:    a.AgentID,
					Address:    a.Address,
					Domain:     a.Domain,
					Status:     string(a.Status),
					Reputation: a.Reputation,
					Tasks:      a.TasksComplete,
					Generation: a.Generation,
				})
			}

			result := map[string]interface{}{
				"total":  len(filtered),
				"agents": filtered,
			}
			if domain != "" {
				result["domain_filter"] = domain
			}
			if statusFilter != "" {
				result["status_filter"] = statusFilter
			}

			return printJSON(cmd, result)
		},
	}

	cmd.Flags().String("domain", "", "Filter by domain")
	cmd.Flags().String("status", "", "Filter by status: active, suspended, retired")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

type agentListEntry struct {
	AgentID    string `json:"agent_id"`
	Address    string `json:"address"`
	Domain     string `json:"domain"`
	Status     string `json:"status"`
	Reputation string `json:"reputation"`
	Tasks      uint64 `json:"tasks_complete"`
	Generation uint64 `json:"generation"`
}

// queryAgentsBySubspace performs an ABCI subspace query on the agent identity prefix
// to iterate all stored agents. The response is proto-encoded KV pairs.
func queryAgentsBySubspace(clientCtx client.Context) ([]types.AgentIdentity, error) {
	resp, err := clientCtx.QueryABCI(abci.RequestQuery{
		Path: fmt.Sprintf("store/%s/subspace", types.StoreKey),
		Data: types.AgentIdentityPrefix,
	})
	if err != nil {
		return nil, fmt.Errorf("ABCI query failed: %w", err)
	}
	if len(resp.Value) == 0 {
		return nil, nil
	}

	// The IAVL subspace response is proto-encoded kv.Pairs.
	// Decode using a minimal proto parser for the Pairs message:
	//   message Pairs { repeated Pair pairs = 1; }
	//   message Pair  { bytes key = 1; bytes value = 2; }
	pairs, err := decodeKVPairs(resp.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to decode subspace response: %w", err)
	}

	var agents []types.AgentIdentity
	for _, p := range pairs {
		var agent types.AgentIdentity
		if json.Unmarshal(p.value, &agent) == nil && agent.AgentID != "" {
			agents = append(agents, agent)
		}
	}
	return agents, nil
}

// kvPairRaw is a minimal KV pair for decoding ABCI subspace responses.
type kvPairRaw struct {
	key   []byte
	value []byte
}

// decodeKVPairs decodes proto-encoded kv.Pairs from an ABCI subspace response.
// Proto wire format: field 1 (Pairs) is repeated, each is a length-delimited Pair message.
// Inside each Pair: field 1 = key (bytes), field 2 = value (bytes).
func decodeKVPairs(data []byte) ([]kvPairRaw, error) {
	var pairs []kvPairRaw
	pos := 0
	for pos < len(data) {
		// Read field tag + wire type.
		if pos >= len(data) {
			break
		}
		tag := data[pos]
		pos++
		fieldNum := tag >> 3
		wireType := tag & 0x07

		if fieldNum != 1 || wireType != 2 {
			// Skip unknown fields.
			if wireType == 0 {
				// Varint: skip bytes until MSB is 0.
				for pos < len(data) && data[pos]&0x80 != 0 {
					pos++
				}
				pos++
			} else if wireType == 2 {
				// Length-delimited: read length, skip.
				length, n := decodeVarint(data[pos:])
				pos += n + int(length)
			} else {
				return nil, fmt.Errorf("unsupported wire type %d at pos %d", wireType, pos)
			}
			continue
		}

		// Read length of the Pair message.
		length, n := decodeVarint(data[pos:])
		pos += n
		if pos+int(length) > len(data) {
			return nil, fmt.Errorf("truncated pair at pos %d", pos)
		}

		// Decode the inner Pair message.
		pairData := data[pos : pos+int(length)]
		pos += int(length)

		pair := kvPairRaw{}
		innerPos := 0
		for innerPos < len(pairData) {
			innerTag := pairData[innerPos]
			innerPos++
			innerField := innerTag >> 3
			innerWire := innerTag & 0x07

			if innerWire != 2 {
				if innerWire == 0 {
					for innerPos < len(pairData) && pairData[innerPos]&0x80 != 0 {
						innerPos++
					}
					innerPos++
				}
				continue
			}

			fLen, fN := decodeVarint(pairData[innerPos:])
			innerPos += fN

			fieldBytes := pairData[innerPos : innerPos+int(fLen)]
			innerPos += int(fLen)

			switch innerField {
			case 1:
				pair.key = fieldBytes
			case 2:
				pair.value = fieldBytes
			}
		}
		pairs = append(pairs, pair)
	}
	return pairs, nil
}

// decodeVarint decodes a protobuf varint from data, returning value and bytes consumed.
func decodeVarint(data []byte) (uint64, int) {
	var val uint64
	var shift uint
	for i, b := range data {
		val |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			return val, i + 1
		}
		shift += 7
		if shift >= 64 {
			break
		}
	}
	return val, len(data)
}

// CmdAgentStatus shows agent health: identity, balance, reputation, tasks, earnings.
func CmdAgentStatus() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-status [agent-id]",
		Short: "Show agent health: balance, reputation, tasks, earnings",
		Long: `Query detailed status for a specific agent by ID.

Example:
  zeroned q knowledge agent-status <agent-id>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			agentID := args[0]

			// Point lookup for agent identity.
			bz, _, err := clientCtx.QueryStore(types.AgentIdentityKey(agentID), types.StoreKey)
			if err != nil {
				return fmt.Errorf("failed to query agent: %w", err)
			}
			if len(bz) == 0 {
				return fmt.Errorf("agent %s not found", agentID)
			}

			var agent types.AgentIdentity
			if err := json.Unmarshal(bz, &agent); err != nil {
				return fmt.Errorf("failed to parse agent data: %w", err)
			}

			result := map[string]interface{}{
				"agent_id":       agent.AgentID,
				"model_id":       agent.ModelID,
				"address":        agent.Address,
				"domain":         agent.Domain,
				"generation":     agent.Generation,
				"status":         string(agent.Status),
				"reputation":     agent.Reputation,
				"tasks_complete": agent.TasksComplete,
				"earnings_total": agent.EarningsTotal,
				"initial_stake":  agent.InitialStake,
				"promoted_at":    agent.PromotedAt,
				"sponsor":        agent.SponsorAddr,
				"can_submit":     agent.CanSubmit,
				"can_review":     agent.CanReview,
				"can_train":      agent.CanTrain,
			}

			if agent.SuspendedAt > 0 {
				result["suspended_at"] = agent.SuspendedAt
			}
			if agent.ParentAgentID != "" {
				result["parent_agent_id"] = agent.ParentAgentID
			}
			if len(agent.Lineage) > 0 {
				result["lineage"] = agent.Lineage
			}

			// Try to fetch reputation per domain.
			if agent.Domain != "" {
				repBz, _, repErr := clientCtx.QueryStore(
					types.AgentDomainReputationKey(agent.Address, agent.Domain), types.StoreKey,
				)
				if repErr == nil && len(repBz) > 0 {
					var rep types.AgentDomainReputation
					if json.Unmarshal(repBz, &rep) == nil {
						result["domain_reputation"] = map[string]string{
							"domain":     rep.DomainID,
							"score":      rep.Score,
							"peak_score": rep.PeakScore,
						}
					}
				}
			}

			return printJSON(cmd, result)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
