"""Data quality filters applied before training format conversion."""

from __future__ import annotations

import hashlib
import re
from typing import Sequence

from .formats import Sample

# Minimum content lengths by type
MIN_CONTENT_LENGTH = 50
MIN_CONTENT_LENGTH_SHORT = 20  # for annotations, corrections


def filter_min_length(sample: Sample) -> bool:
    """Return True if sample meets minimum content length."""
    threshold = MIN_CONTENT_LENGTH
    if sample.sample_type.lower() in ("annotation", "correction"):
        threshold = MIN_CONTENT_LENGTH_SHORT
    return len(sample.content.strip()) >= threshold


def filter_language_match(sample: Sample) -> bool:
    """Basic language verification — checks for expected script patterns."""
    content = sample.content.strip()
    if not content:
        return False

    lang = sample.language.lower()
    if lang in ("en", "english"):
        # Check that content is predominantly ASCII/Latin
        ascii_ratio = sum(1 for c in content if ord(c) < 128) / len(content)
        return ascii_ratio > 0.7
    # For other languages, accept by default (real verification needs a model)
    return True


def estimate_tokens(content: str) -> int:
    """Rough token count estimation (words * 1.3 for English)."""
    words = len(content.split())
    return int(words * 1.3)


# ── Near-duplicate detection via SimHash ─────────────────────────────────────

def _ngrams(text: str, n: int = 3) -> list[str]:
    """Extract character n-grams from normalized text."""
    text = re.sub(r"[^a-z0-9 ]", "", text.lower())
    words = text.split()
    if len(words) < n:
        return words
    return [" ".join(words[i : i + n]) for i in range(len(words) - n + 1)]


def simhash(text: str, bits: int = 64) -> int:
    """Compute SimHash fingerprint."""
    vec = [0] * bits
    for gram in _ngrams(text):
        h = int(hashlib.md5(gram.encode()).hexdigest(), 16)
        for i in range(bits):
            if h & (1 << i):
                vec[i] += 1
            else:
                vec[i] -= 1
    result = 0
    for i in range(bits):
        if vec[i] > 0:
            result |= 1 << i
    return result


def hamming_distance(a: int, b: int) -> int:
    """Count differing bits between two integers."""
    return bin(a ^ b).count("1")


class DedupFilter:
    """Near-duplicate detection using SimHash with configurable threshold."""

    def __init__(self, threshold: int = 6):
        self.threshold = threshold
        self.seen: dict[str, int] = {}  # id → simhash

    def is_duplicate(self, sample: Sample) -> bool:
        """Return True if sample is a near-duplicate of an already-seen sample."""
        h = simhash(sample.content)
        for seen_id, seen_hash in self.seen.items():
            if hamming_distance(h, seen_hash) <= self.threshold:
                return True
        self.seen[sample.id] = h
        return False


def apply_filters(
    samples: Sequence[Sample],
    dedup: bool = True,
    min_length: bool = True,
    lang_check: bool = True,
) -> list[Sample]:
    """Apply all quality filters and return passing samples."""
    result = []
    dedup_filter = DedupFilter() if dedup else None

    for s in samples:
        if min_length and not filter_min_length(s):
            continue
        if lang_check and not filter_language_match(s):
            continue
        if dedup_filter and dedup_filter.is_duplicate(s):
            continue
        result.append(s)

    return result
