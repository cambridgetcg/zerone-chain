package keeper

import "github.com/zerone-chain/zerone/x/knowledge/types"

// SetHooks registers a KnowledgeHooks consumer. Panics if hooks
// were already set — chain via MultiKnowledgeHooks if multiple
// consumers are required. Called once at app init.
func (k *Keeper) SetHooks(h types.KnowledgeHooks) *Keeper {
	if k.hooks != nil {
		panic("x/knowledge: KnowledgeHooks already set; use MultiKnowledgeHooks to chain consumers")
	}
	k.hooks = h
	return k
}

// Hooks returns the registered hooks consumer, or a no-op multi-hooks
// if none has been registered. Always safe to call.
func (k *Keeper) Hooks() types.KnowledgeHooks {
	if k.hooks == nil {
		return types.MultiKnowledgeHooks{}
	}
	return k.hooks
}
