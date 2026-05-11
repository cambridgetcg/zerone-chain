package keeper

import (
	"context"
	"fmt"

	creedtypes "github.com/zerone-chain/zerone/x/creed/types"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// LoadDoctrineFacts materializes every commitment + mechanism + axis
// from all four doctrines (TRUTH_SEEKING, TOK_SUBSTRATE, USEFUL_WORK,
// STRANGE_LOOP) as verified Facts in x/knowledge. Also creates the
// cross-doctrine "Echoes:" edges via SetFactRelation. Idempotent:
// re-running does not overwrite existing Facts.
//
// Called from x/knowledge.InitGenesis after domain creation +
// methodology seeding + normative commitment seeding. Also exposed
// for upgrade handlers if SL-α ships against an already-running chain.
//
// Doctrinal status: VERIFIED at genesis, Confidence=1M, AxiomDistance=0
// — the "privileged but verifiable" ontology per the SL-α design.
func (k Keeper) LoadDoctrineFacts(ctx context.Context) error {
	// 1. Create the four doctrine domains (idempotent).
	domains := []string{
		types.DoctrineDomainTruthSeeking,
		types.DoctrineDomainToK,
		types.DoctrineDomainUsefulWork,
		types.DoctrineDomainStrangeLoop,
	}
	for _, dom := range domains {
		if _, ok := k.GetDomain(ctx, dom); ok {
			continue
		}
		if err := k.SetDomain(ctx, &types.Domain{
			Name:   dom,
			Status: types.DomainStatus_DOMAIN_STATUS_ACTIVE,
		}); err != nil {
			return fmt.Errorf("create doctrine domain %s: %w", dom, err)
		}
	}

	// 2. Truth-seeking commitments 1-20.
	for _, c := range creedtypes.CanonicalCommitments {
		f := types.BuildDoctrineFact(
			fmt.Sprintf("commitment-%d", c.Number),
			types.DoctrineDomainTruthSeeking,
			c.Name,
		)
		if err := k.SetFactIfAbsent(ctx, f); err != nil {
			return fmt.Errorf("write truth-seeking commitment %d: %w", c.Number, err)
		}
	}

	// 3. ToK commitments TC1-TC6.
	for _, c := range creedtypes.CanonicalToKCommitments {
		f := types.BuildDoctrineFact(
			fmt.Sprintf("commitment-%s", c.Number),
			types.DoctrineDomainToK,
			c.Name,
		)
		if err := k.SetFactIfAbsent(ctx, f); err != nil {
			return fmt.Errorf("write ToK commitment %s: %w", c.Number, err)
		}
	}

	// 4. Useful-work: UW + mechanisms + axes.
	uwFact := types.BuildDoctrineFact(
		"commitment-UW",
		types.DoctrineDomainUsefulWork,
		creedtypes.UsefulWorkStatement,
	)
	if err := k.SetFactIfAbsent(ctx, uwFact); err != nil {
		return fmt.Errorf("write UW commitment: %w", err)
	}
	for _, m := range creedtypes.CanonicalUsefulWorkMechanisms {
		f := types.BuildDoctrineFact(
			fmt.Sprintf("mechanism-UW-M%d", m.Number),
			types.DoctrineDomainUsefulWork,
			m.Name,
		)
		if err := k.SetFactIfAbsent(ctx, f); err != nil {
			return fmt.Errorf("write UW mechanism M%d: %w", m.Number, err)
		}
	}
	for _, axis := range creedtypes.CanonicalRecursiveAxes {
		f := types.BuildDoctrineFact(
			fmt.Sprintf("axis-%s", axis),
			types.DoctrineDomainUsefulWork,
			axis,
		)
		if err := k.SetFactIfAbsent(ctx, f); err != nil {
			return fmt.Errorf("write UW axis %s: %w", axis, err)
		}
	}

	// 5. Strange-loop: SL + mechanisms.
	slFact := types.BuildDoctrineFact(
		"commitment-SL",
		types.DoctrineDomainStrangeLoop,
		creedtypes.StrangeLoopStatement,
	)
	if err := k.SetFactIfAbsent(ctx, slFact); err != nil {
		return fmt.Errorf("write SL commitment: %w", err)
	}
	for _, m := range creedtypes.CanonicalStrangeLoopMechanisms {
		f := types.BuildDoctrineFact(
			fmt.Sprintf("mechanism-SL-M%d", m.Number),
			types.DoctrineDomainStrangeLoop,
			m.Name,
		)
		if err := k.SetFactIfAbsent(ctx, f); err != nil {
			return fmt.Errorf("write SL mechanism M%d: %w", m.Number, err)
		}
	}

	// 6. Cross-doctrine "Echoes:" edges (eager at genesis).
	for _, e := range creedtypes.CanonicalDoctrineEchoes {
		if err := k.SetFactRelation(ctx, &types.FactRelation{
			SourceFactId: e.From,
			TargetFactId: e.To,
			Relation:     e.Relation,
		}); err != nil {
			return fmt.Errorf("write doctrine echo %s→%s: %w", e.From, e.To, err)
		}
	}

	return nil
}
