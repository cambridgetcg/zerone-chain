"""CLI for the training pipeline."""

from __future__ import annotations

import json
import os
import sys

import click

from .formats import Sample
from .pipeline import load_samples_from_db, run_pipeline


@click.group()
def main():
    """ZERONE Training Pipeline — transform samples into training formats."""
    pass


@main.command()
@click.option("--snapshot", required=True, help="Snapshot version to transform")
@click.option("--format", "fmt", default=None, help="Force format (chat, completion, dpo)")
@click.option("--domain", default=None, help="Filter by domain")
@click.option("--quality", default=None, help="Filter by quality tier")
@click.option("--language", default=None, help="Filter by language")
@click.option("--output", required=True, help="Output directory")
@click.option("--dsn", default=None, help="PostgreSQL DSN (default: DATABASE_URL env)")
@click.option("--no-split", is_flag=True, help="Skip train/val/test splitting")
@click.option("--domain-context/--no-domain-context", default=True, help="Include domain in system prompts")
def run(snapshot, fmt, domain, quality, language, output, dsn, no_split, domain_context):
    """Transform a snapshot into training-ready JSONL files."""
    dsn = dsn or os.environ.get("DATABASE_URL", "")
    if not dsn:
        click.echo("Error: --dsn or DATABASE_URL required", err=True)
        sys.exit(1)

    click.echo(f"Loading samples from snapshot {snapshot}...")
    samples = load_samples_from_db(dsn, snapshot_version=snapshot, domain=domain, quality=quality, language=language)

    if not samples:
        click.echo("No samples found matching filters.")
        return

    click.echo(f"Loaded {len(samples)} samples. Running pipeline...")
    counts = run_pipeline(
        samples,
        output_dir=output,
        format_override=fmt,
        domain_context=domain_context,
        split=not no_split,
    )

    for split_name, count in counts.items():
        click.echo(f"  {split_name}: {count} samples")
    click.echo(f"Output written to {output}/")


@main.command()
@click.option("--snapshot", required=True, help="Snapshot version")
@click.option("--dsn", default=None, help="PostgreSQL DSN")
def stats(snapshot, dsn):
    """Show sample statistics for a snapshot."""
    dsn = dsn or os.environ.get("DATABASE_URL", "")
    if not dsn:
        click.echo("Error: --dsn or DATABASE_URL required", err=True)
        sys.exit(1)

    samples = load_samples_from_db(dsn, snapshot_version=snapshot)
    if not samples:
        click.echo("No samples found.")
        return

    # Count by type
    by_type: dict[str, int] = {}
    by_domain: dict[str, int] = {}
    by_quality: dict[str, int] = {}
    for s in samples:
        by_type[s.sample_type] = by_type.get(s.sample_type, 0) + 1
        by_domain[s.domain] = by_domain.get(s.domain, 0) + 1
        by_quality[s.quality_tier] = by_quality.get(s.quality_tier, 0) + 1

    click.echo(f"\nSnapshot: {snapshot} ({len(samples)} samples)\n")

    click.echo("By Type:")
    for k, v in sorted(by_type.items(), key=lambda x: -x[1]):
        click.echo(f"  {k:20s} {v:6d}")

    click.echo("\nBy Domain:")
    for k, v in sorted(by_domain.items(), key=lambda x: -x[1]):
        click.echo(f"  {k:20s} {v:6d}")

    click.echo("\nBy Quality:")
    for k, v in sorted(by_quality.items(), key=lambda x: -x[1]):
        click.echo(f"  {k:20s} {v:6d}")


if __name__ == "__main__":
    main()
