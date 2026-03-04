package simulation

// The alignment module only has MsgUpdateParams (authority-gated) and
// MsgActivate (authority-gated). Neither is suitable for random simulation
// since they require governance authority. This package exists as a placeholder
// so the module's simulation directory is consistent with the others.
//
// Alignment observations and corrections are computed automatically in
// BeginBlock, so they are exercised implicitly by any simulation run.
