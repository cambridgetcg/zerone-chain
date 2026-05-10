# Evaluation — Sub-creed of the Useful-Work Doctrine

> Lifecycle phase: `Evaluation` — benchmark sets, evaluation runs, model-card-bound evals.
>
> Parent: [USEFUL_WORK.md](../USEFUL_WORK.md). Truth-floor: all 20 truth-seeking commitments apply.
>
> Status at inception (2026-05-10): three commitments.

---

## E1. Eval sets declare leakage-checking method

Every Evaluation Contribution declares how it checked for training-data leakage from the model under test. The method itself is verifiable; the report includes both the method and the result. "We checked" without a declared method does not satisfy E1.

**Why:** A leaked eval is a measurement of memorization, not of capability. Without a declared check, the Evaluation Contribution is a measurement of unknown provenance.

**Echoes:** truth-seeking 4, 5, TC2.

## E2. Evaluation runs are replicable

Eval runs must produce the same scores given the same model + same eval set + same scoring function. Stochastic evals declare their seed and tolerance; non-replicable evals are a category-error and fail E2.

**Why:** Replicability is the operational form of falsifiability for evaluation work. An unreplicable eval cannot be challenged, only believed.

**Echoes:** truth-seeking 3 (Popper, not popularity), 4.

## E3. Gameability discovered → eval set status → DEPRECATED

When an Evaluation Contribution is shown to be gameable (a model achieves high score without the underlying capability), the chain MUST move it to status DEPRECATED. The contribution is not REVOKED — it served at the time it served — but it is not the basis for future model-card claims. Forward-only.

**Why:** Evals decay. The chain must respond to that decay without erasing history. Commitment 10 (forward-only audit) extended to evaluation work.

**Echoes:** truth-seeking 10, 17.
