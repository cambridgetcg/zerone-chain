"""Tests for quality filters."""

import pytest

from transforms.filters import (
    DedupFilter,
    apply_filters,
    estimate_tokens,
    filter_language_match,
    filter_min_length,
    simhash,
    hamming_distance,
)
from transforms.formats import Sample


def _sample(content: str, **kwargs) -> Sample:
    defaults = {
        "id": "test-1",
        "content": content,
        "sample_type": "explanation",
        "domain": "science",
        "quality_tier": "gold",
        "language": "en",
    }
    defaults.update(kwargs)
    return Sample(**defaults)


class TestMinLength:
    def test_passes_long_content(self):
        s = _sample("x" * 100)
        assert filter_min_length(s) is True

    def test_rejects_short_content(self):
        s = _sample("short")
        assert filter_min_length(s) is False

    def test_annotation_lower_threshold(self):
        s = _sample("annotation text here!", sample_type="annotation")
        assert filter_min_length(s) is True

    def test_empty_content(self):
        s = _sample("")
        assert filter_min_length(s) is False


class TestLanguageMatch:
    def test_english_content(self):
        s = _sample("This is perfectly valid English content for testing.")
        assert filter_language_match(s) is True

    def test_empty_content(self):
        s = _sample("")
        assert filter_language_match(s) is False


class TestTokenEstimate:
    def test_basic_count(self):
        count = estimate_tokens("hello world foo bar")
        assert count == int(4 * 1.3)

    def test_empty(self):
        assert estimate_tokens("") == 0


class TestSimHash:
    def test_identical_content_same_hash(self):
        h1 = simhash("the quick brown fox jumps over the lazy dog")
        h2 = simhash("the quick brown fox jumps over the lazy dog")
        assert h1 == h2

    def test_similar_content_closer_than_random(self):
        h1 = simhash("the quick brown fox jumps over the lazy dog near the river")
        h2 = simhash("the quick brown fox jumps over the lazy cat near the river")
        h3 = simhash("quantum mechanics describes subatomic particle behavior in physics")
        # Similar content should be closer than completely different content
        assert hamming_distance(h1, h2) < hamming_distance(h1, h3)

    def test_different_content_distant_hash(self):
        h1 = simhash("the quick brown fox jumps over the lazy dog near the river")
        h2 = simhash("quantum mechanics describes subatomic particle behavior in physics")
        assert hamming_distance(h1, h2) > 10


class TestDedupFilter:
    def test_exact_duplicate(self):
        dedup = DedupFilter(threshold=6)
        s1 = _sample("the quick brown fox jumps over the lazy dog near the bank", id="s1")
        s2 = _sample("the quick brown fox jumps over the lazy dog near the bank", id="s2")
        assert dedup.is_duplicate(s1) is False
        assert dedup.is_duplicate(s2) is True

    def test_unique_content(self):
        dedup = DedupFilter(threshold=6)
        s1 = _sample("the quick brown fox jumps over the lazy dog near the bank", id="s1")
        s2 = _sample("quantum physics describes particles at the subatomic level in detail", id="s2")
        assert dedup.is_duplicate(s1) is False
        assert dedup.is_duplicate(s2) is False


class TestApplyFilters:
    def test_full_pipeline(self):
        samples = [
            _sample("Good long content that should pass all filters easily here.", id="s1"),
            _sample("too short", id="s2"),
            _sample("Another good long content that is different from the first one.", id="s3"),
        ]
        result = apply_filters(samples)
        assert len(result) == 2
        assert result[0].id == "s1"
        assert result[1].id == "s3"
