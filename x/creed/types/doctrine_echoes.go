package types

import (
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// DoctrineEcho is one cross-doctrine relation.
type DoctrineEcho struct {
	From     string
	To       string
	Relation knowledgetypes.RelationType
}

// CanonicalDoctrineEchoes is the hand-curated set of cross-doctrine
// "Echoes:" relations from the actual doctrine markdown files.
//
// Re-curate whenever a doctrine's "Echoes:" section is updated; the
// cross-stack TestStrangeLoop_DoctrineAndContractStayInSync meta-test
// detects drift.
//
// Curation source: grep -A 8 "^\*\*Echoes:\*\*" docs/TRUTH_SEEKING.md
// docs/TOK_SUBSTRATE.md docs/USEFUL_WORK.md docs/STRANGE_LOOP.md
// Intra-doctrine echoes (source and target in the same doctrine) are
// intentionally excluded; this list contains only cross-doctrine edges.
var CanonicalDoctrineEchoes = []DoctrineEcho{
	// ── TOK_SUBSTRATE.md cross-doctrine echoes ────────────────────────────
	// TC1 echoes: TC2, TC3 (intra — excluded), commitment-13 (cross)
	{"commitment-TC1", "commitment-13", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},

	// TC2 echoes: TC1, TC4 (intra — excluded), commitment-10, commitment-13 (cross)
	{"commitment-TC2", "commitment-10", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
	{"commitment-TC2", "commitment-13", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},

	// TC3 echoes: TC1, TC4 (intra — excluded), commitment-14 (cross)
	{"commitment-TC3", "commitment-14", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},

	// TC4 echoes: TC2, TC3 (intra — excluded), commitment-3, commitment-10 (cross)
	{"commitment-TC4", "commitment-3", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
	{"commitment-TC4", "commitment-10", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},

	// TC5 echoes: TC1, TC4 (intra — excluded), commitment-11, commitment-6 (cross)
	{"commitment-TC5", "commitment-11", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
	{"commitment-TC5", "commitment-6", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},

	// TC6 echoes: TC1, TC3 (intra — excluded), commitment-12, commitment-13 (cross)
	{"commitment-TC6", "commitment-12", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
	{"commitment-TC6", "commitment-13", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},

	// ── USEFUL_WORK.md cross-doctrine echoes ──────────────────────────────
	// UW commitment echoes (from "The single commitment — UW" Echoes: block
	// and Graph layer section): commitment-11, commitment-12, TC1, TC6
	{"commitment-UW", "commitment-11", knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS},
	{"commitment-UW", "commitment-12", knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS},
	{"commitment-UW", "commitment-TC1", knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS},
	{"commitment-UW", "commitment-TC6", knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS},

	// Per-mechanism cross-references (Graph layer: M2←TC2, M3←commitment-6,
	// M4←TC6, M5←commitment-14, M6←TC6, M7←commitment-12)
	{"mechanism-UW-M2", "commitment-TC2", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
	{"mechanism-UW-M3", "commitment-6", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
	{"mechanism-UW-M4", "commitment-TC6", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
	{"mechanism-UW-M5", "commitment-14", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
	{"mechanism-UW-M6", "commitment-TC6", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
	{"mechanism-UW-M7", "commitment-12", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},

	// ── STRANGE_LOOP.md cross-doctrine echoes ─────────────────────────────
	// SL commitment echoes: UW, commitment-10, commitment-12, TC6
	// SL REFINES UW (SL is what UW becomes at its operational limit)
	{"commitment-SL", "commitment-UW", knowledgetypes.RelationType_RELATION_TYPE_REFINES},
	// SL REQUIRES commitment-10 (forward-only audit — superseded doctrine Facts queryable forever)
	{"commitment-SL", "commitment-10", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
	// SL REFINES commitment-12 (chain pays for own audit — extended to authorship)
	{"commitment-SL", "commitment-12", knowledgetypes.RelationType_RELATION_TYPE_REFINES},
	// SL REFINES TC6 (lineage flows back — extended to everyone, including protocol authors)
	{"commitment-SL", "commitment-TC6", knowledgetypes.RelationType_RELATION_TYPE_REFINES},
}
