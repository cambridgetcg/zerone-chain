"""Dataset splitting with stratification by domain and quality tier."""

from __future__ import annotations

import hashlib
from collections import defaultdict
from typing import Sequence

from .formats import Sample


def split_dataset(
    samples: Sequence[Sample],
    train_ratio: float = 0.90,
    val_ratio: float = 0.05,
    test_ratio: float = 0.05,
    seed: int = 42,
) -> tuple[list[Sample], list[Sample], list[Sample]]:
    """Split samples into train/val/test with stratification by domain+quality.

    Uses deterministic hashing so the same sample always lands in the same split,
    regardless of dataset ordering or other samples present.
    """
    assert abs(train_ratio + val_ratio + test_ratio - 1.0) < 1e-6

    train, val, test = [], [], []

    # Group by stratum (domain × quality_tier)
    strata: dict[str, list[Sample]] = defaultdict(list)
    for s in samples:
        key = f"{s.domain}|{s.quality_tier}"
        strata[key].append(s)

    for _key, group in sorted(strata.items()):
        # Sort deterministically within stratum
        group.sort(key=lambda s: s.id)

        for s in group:
            bucket = _deterministic_bucket(s.id, seed)
            if bucket < train_ratio:
                train.append(s)
            elif bucket < train_ratio + val_ratio:
                val.append(s)
            else:
                test.append(s)

    return train, val, test


def _deterministic_bucket(sample_id: str, seed: int) -> float:
    """Map a sample ID to [0, 1) deterministically."""
    h = hashlib.sha256(f"{seed}:{sample_id}".encode()).hexdigest()
    return int(h[:8], 16) / 0xFFFFFFFF


def quality_weight(sample: Sample) -> float:
    """Return sampling weight based on quality tier.

    Gold samples are upweighted to appear more frequently in training.
    """
    weights = {"gold": 3.0, "silver": 1.5, "bronze": 1.0}
    return weights.get(sample.quality_tier, 1.0)
