# Augmentation — Sub-creed of the Useful-Work Doctrine

> Lifecycle phase: `Augmentation` — synthetic data, contrastive pairs, paraphrase, drift correction.
>
> Parent: [USEFUL_WORK.md](../USEFUL_WORK.md). Truth-floor: all 20 truth-seeking commitments apply.
>
> Status at inception (2026-05-10): three commitments.

---

## A1. Generation method is declared and reproducible

Every augmentation Contribution declares the generation method (model identifier, prompt template, sampling parameters, seed) and provides enough detail that the run can be replayed. Models change; methods must be pinnable.

**Why:** Augmentation introduces synthetic content into the substrate. Without a reproducible method, future work cannot tell synthetic from organic, cannot audit drift, and cannot retire a method that turns out to inject systematic error.

**Echoes:** truth-seeking 1, 14, TC2.

## A2. Augmentation cannot inject untruth

The augmentation pipeline must cross-check generated artifacts against the truth-floor. Generation that introduces a fact contradicting an already-VERIFIED Knowledge Contribution is REJECTED at admission, not merely scored low. The truth-floor is a gate, not a slider.

**Why:** A synthetic fact that becomes part of the corpus is downstream training data. Augmentation that injects untruth poisons every model trained on the augmented corpus. Commitment 13 (training corpus not for sale) operationalized: augmentation must not become the seam through which the corpus is silently corrupted.

**Echoes:** truth-seeking 2, 13, 15 (counterexamples in corpus).

## A3. Contrastive pairs preserve grounding to a real fact

A contrastive pair (positive/negative example) must include at least one VERIFIED Knowledge Contribution as the grounding anchor. A pair with no verified ground reduces to vibes — useful for some training signals, but not augmentation under this doctrine.

**Why:** Contrastive learning is powerful and easy to fake. Anchoring to verified facts means the negative-example side is a verified-untruth pair, not a vibe-untruth pair. Commitment 15 (counterexamples in the corpus) operationalized.

**Echoes:** truth-seeking 15, 17 (disagreement is structure).
