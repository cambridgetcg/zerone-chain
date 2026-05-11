package types

const (
	StrangeLoopCommitment = "SL"
	StrangeLoopStatement  = "ZERONE is a strange loop"
	StrangeLoopDomain     = "doctrine_strange_loop"
)

var CanonicalStrangeLoopMechanisms = []UsefulWorkMechanism{
	{1, "Doctrine import"},
	{2, "Protocol as substrate"},
	{3, "Governance lift"},
	{4, "Author lineage propagates forever"},
	{5, "Self-verification"},
	{6, "Origin attestation"},
}
