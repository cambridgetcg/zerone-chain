"""Tests for dataset splitting."""

import pytest

from transforms.formats import Sample
from transforms.splits import quality_weight, split_dataset


def _sample(id: str, domain: str = "science", quality: str = "gold") -> Sample:
    return Sample(
        id=id,
        content=f"Content for {id}",
        sample_type="explanation",
        domain=domain,
        quality_tier=quality,
    )


class TestSplitDataset:
    def test_all_samples_assigned(self):
        samples = [_sample(f"s-{i}") for i in range(100)]
        train, val, test = split_dataset(samples)
        assert len(train) + len(val) + len(test) == 100

    def test_approximate_ratios(self):
        samples = [_sample(f"s-{i}") for i in range(1000)]
        train, val, test = split_dataset(samples)
        assert 850 <= len(train) <= 950
        assert 20 <= len(val) <= 80
        assert 20 <= len(test) <= 80

    def test_deterministic(self):
        samples = [_sample(f"s-{i}") for i in range(100)]
        train1, val1, test1 = split_dataset(samples, seed=42)
        train2, val2, test2 = split_dataset(samples, seed=42)
        assert [s.id for s in train1] == [s.id for s in train2]
        assert [s.id for s in val1] == [s.id for s in val2]

    def test_different_seed_different_split(self):
        samples = [_sample(f"s-{i}") for i in range(100)]
        train1, _, _ = split_dataset(samples, seed=42)
        train2, _, _ = split_dataset(samples, seed=99)
        # Very unlikely to be identical with different seeds
        assert [s.id for s in train1] != [s.id for s in train2]

    def test_stratification(self):
        samples = [
            _sample(f"sci-{i}", domain="science", quality="gold") for i in range(50)
        ] + [
            _sample(f"lit-{i}", domain="literature", quality="silver") for i in range(50)
        ]
        train, val, test = split_dataset(samples)
        # Both domains should appear in train
        train_domains = {s.domain for s in train}
        assert "science" in train_domains
        assert "literature" in train_domains


class TestQualityWeight:
    def test_gold(self):
        assert quality_weight(_sample("x", quality="gold")) == 3.0

    def test_silver(self):
        assert quality_weight(_sample("x", quality="silver")) == 1.5

    def test_bronze(self):
        assert quality_weight(_sample("x", quality="bronze")) == 1.0

    def test_unknown(self):
        assert quality_weight(_sample("x", quality="unknown")) == 1.0
