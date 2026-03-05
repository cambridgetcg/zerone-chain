"""CLI for the fine-tuning pipeline."""

from __future__ import annotations

import sys

import click

from .config import TrainingConfig
from .registry import ModelRegistry
from .runner import Evaluator, TrainingRunner


@click.group()
def main():
    """ZERONE Fine-Tuning Pipeline — train, evaluate, and manage models."""
    pass


@main.command()
@click.option("--config", required=True, help="Path to training config YAML")
@click.option("--registry", default="model_registry.json", help="Path to model registry")
def run(config, registry):
    """Run a fine-tuning job."""
    cfg = TrainingConfig.from_yaml(config)
    reg = ModelRegistry(registry)
    runner = TrainingRunner(cfg, reg)

    click.echo(f"Starting training: {runner.model_id}")
    click.echo(f"  Base model: {cfg.base_model}")
    click.echo(f"  Method: {cfg.method}")
    click.echo(f"  Dataset: {cfg.dataset.path}")

    metrics = runner.run()

    click.echo(f"\nTraining complete!")
    for k, v in metrics.items():
        click.echo(f"  {k}: {v}")


@main.command()
@click.option("--checkpoint", required=True, help="Path to checkpoint directory")
@click.option("--config", required=True, help="Path to training config YAML")
@click.option("--registry", default="model_registry.json", help="Path to model registry")
def resume(checkpoint, config, registry):
    """Resume training from a checkpoint."""
    cfg = TrainingConfig.from_yaml(config)
    reg = ModelRegistry(registry)
    runner = TrainingRunner(cfg, reg)

    click.echo(f"Resuming training from: {checkpoint}")
    metrics = runner.resume(checkpoint)

    click.echo(f"\nTraining complete!")
    for k, v in metrics.items():
        click.echo(f"  {k}: {v}")


@main.command()
@click.option("--model", required=True, help="Model ID to evaluate")
@click.option("--test-data", default=None, help="Path to test data JSONL")
@click.option("--registry", default="model_registry.json", help="Path to model registry")
def eval(model, test_data, registry):
    """Evaluate a trained model."""
    reg = ModelRegistry(registry)
    evaluator = Evaluator(reg)

    click.echo(f"Evaluating: {model}")
    scores = evaluator.evaluate(model, test_data)

    if scores:
        for k, v in scores.items():
            click.echo(f"  {k}: {v:.4f}")
    else:
        click.echo("  No scores computed (missing test data or dependencies)")


@main.command()
@click.option("--model", required=True, help="Model ID to promote")
@click.option("--registry", default="model_registry.json", help="Path to model registry")
def promote(model, registry):
    """Mark a model as ready for serving."""
    reg = ModelRegistry(registry)
    if reg.update_status(model, "ready"):
        click.echo(f"Model {model} promoted to 'ready'")
    else:
        click.echo(f"Model {model} not found", err=True)
        sys.exit(1)


@main.command("list")
@click.option("--status", default=None, help="Filter by status")
@click.option("--registry", default="model_registry.json", help="Path to model registry")
def list_models(status, registry):
    """List all models in the registry."""
    reg = ModelRegistry(registry)
    models = reg.list_models(status)

    if not models:
        click.echo("No models found.")
        return

    click.echo(f"{'MODEL ID':40s} {'STATUS':10s} {'METHOD':8s} {'BASE MODEL':40s} {'LOSS':8s}")
    for m in models:
        loss = m.metrics.get("final_loss", "N/A")
        loss_str = f"{loss:.4f}" if isinstance(loss, float) else str(loss)
        click.echo(f"{m.model_id:40s} {m.status:10s} {m.method:8s} {m.base_model:40s} {loss_str:8s}")


if __name__ == "__main__":
    main()
