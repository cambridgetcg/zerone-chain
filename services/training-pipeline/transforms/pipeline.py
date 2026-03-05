"""Main transformation pipeline: DB → filtered → formatted → JSONL."""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any, Sequence

from .filters import apply_filters, estimate_tokens
from .formats import Sample, to_training_format
from .splits import quality_weight, split_dataset


def load_samples_from_db(
    dsn: str,
    snapshot_version: str | None = None,
    domain: str | None = None,
    quality: str | None = None,
    language: str | None = None,
) -> list[Sample]:
    """Load samples from the staging PostgreSQL database."""
    import psycopg2

    conn = psycopg2.connect(dsn)
    try:
        cur = conn.cursor()

        if snapshot_version:
            query = """
                SELECT s.id, s.content, s.sample_type, s.domain, s.quality_tier,
                       s.quality_score, s.novelty_score, s.source_uri, s.source_platform,
                       s.original_author, s.language, s.tags, s.thread_id,
                       s.parent_sample_id, s.thread_position
                FROM samples s
                JOIN snapshot_samples ss ON ss.sample_id = s.id
                JOIN dataset_snapshots ds ON ds.id = ss.snapshot_id
                WHERE ds.version = %s
            """
            params: list[Any] = [snapshot_version]
        else:
            query = "SELECT id, content, sample_type, domain, quality_tier, quality_score, novelty_score, source_uri, source_platform, original_author, language, tags, thread_id, parent_sample_id, thread_position FROM samples WHERE 1=1"
            params = []

        if domain:
            query += " AND s.domain = %s" if snapshot_version else " AND domain = %s"
            params.append(domain)
        if quality:
            query += " AND s.quality_tier = %s" if snapshot_version else " AND quality_tier = %s"
            params.append(quality)
        if language:
            query += " AND s.language = %s" if snapshot_version else " AND language = %s"
            params.append(language)

        cur.execute(query, params)
        rows = cur.fetchall()
    finally:
        conn.close()

    samples = []
    for row in rows:
        samples.append(
            Sample(
                id=row[0],
                content=row[1],
                sample_type=row[2],
                domain=row[3],
                quality_tier=row[4],
                quality_score=row[5] or 0,
                novelty_score=row[6] or 0,
                source_uri=row[7] or "",
                source_platform=row[8] or "",
                original_author=row[9] or "",
                language=row[10] or "en",
                tags=row[11] or [],
                thread_id=row[12] or "",
                parent_sample_id=row[13] or "",
                thread_position=row[14] or 0,
            )
        )
    return samples


def reconstruct_threads(samples: list[Sample]) -> list[Sample]:
    """Group threaded samples by thread_id and sort by thread_position.

    Returns samples with thread content concatenated into the root sample.
    Non-threaded samples are returned unchanged.
    """
    threads: dict[str, list[Sample]] = {}
    standalone: list[Sample] = []

    for s in samples:
        if s.thread_id:
            threads.setdefault(s.thread_id, []).append(s)
        else:
            standalone.append(s)

    # Merge thread samples into concatenated content
    for tid, thread_samples in threads.items():
        thread_samples.sort(key=lambda s: s.thread_position)
        merged_content = "\n\n".join(s.content for s in thread_samples)
        root = thread_samples[0]
        root.content = merged_content
        standalone.append(root)

    return standalone


def run_pipeline(
    samples: Sequence[Sample],
    output_dir: str,
    format_override: str | None = None,
    domain_context: bool = True,
    split: bool = True,
    train_ratio: float = 0.90,
    val_ratio: float = 0.05,
    test_ratio: float = 0.05,
) -> dict[str, int]:
    """Run the full transformation pipeline.

    Returns a dict of split_name → sample_count.
    """
    out = Path(output_dir)
    out.mkdir(parents=True, exist_ok=True)

    # Filter
    filtered = apply_filters(list(samples))

    # Reconstruct threads
    filtered = reconstruct_threads(filtered)

    # Split
    if split:
        train, val, test = split_dataset(
            filtered,
            train_ratio=train_ratio,
            val_ratio=val_ratio,
            test_ratio=test_ratio,
        )
        splits = {"train": train, "validation": val, "test": test}
    else:
        splits = {"all": filtered}

    counts: dict[str, int] = {}
    for name, split_samples in splits.items():
        records = []
        for s in split_samples:
            rec = to_training_format(s, domain_context=domain_context)
            rec["_meta"] = {
                "id": s.id,
                "domain": s.domain,
                "quality_tier": s.quality_tier,
                "quality_weight": quality_weight(s),
                "token_estimate": estimate_tokens(s.content),
            }
            records.append(rec)

        path = out / f"{name}.jsonl"
        with open(path, "w", encoding="utf-8") as f:
            for rec in records:
                f.write(json.dumps(rec, ensure_ascii=False) + "\n")

        counts[name] = len(records)

    # Write stats
    stats = {
        "total_input": len(samples),
        "after_filter": len(filtered),
        "splits": counts,
        "domains": _count_by(filtered, "domain"),
        "quality_tiers": _count_by(filtered, "quality_tier"),
        "sample_types": _count_by(filtered, "sample_type"),
    }
    with open(out / "stats.json", "w") as f:
        json.dump(stats, f, indent=2)

    return counts


def _count_by(samples: list[Sample], attr: str) -> dict[str, int]:
    """Count samples by attribute."""
    counts: dict[str, int] = {}
    for s in samples:
        val = getattr(s, attr, "unknown")
        counts[val] = counts.get(val, 0) + 1
    return counts
