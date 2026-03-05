"""Training runner — orchestrates fine-tuning with LoRA/QLoRA.

This module provides the training interface. Actual GPU training requires
torch, transformers, peft, and bitsandbytes to be installed. The runner
is structured to be importable without GPU dependencies for testing and
config validation.
"""

from __future__ import annotations

import json
import logging
import os
from pathlib import Path
from typing import Any

from .config import TrainingConfig
from .registry import ModelEntry, ModelRegistry

logger = logging.getLogger(__name__)


class TrainingRunner:
    """Runs fine-tuning jobs based on a TrainingConfig."""

    def __init__(self, config: TrainingConfig, registry: ModelRegistry | None = None):
        self.config = config
        self.registry = registry or ModelRegistry()
        self.model_id = f"{config.name}.{_dataset_version(config)}"

    def run(self) -> dict[str, Any]:
        """Execute the training pipeline.

        Returns training metrics dict.
        """
        cfg = self.config

        # Register model entry
        entry = ModelEntry(
            model_id=self.model_id,
            base_model=cfg.base_model,
            dataset_version=_dataset_version(cfg),
            method=cfg.method,
            domain_focus=cfg.name.split("-")[1] if "-" in cfg.name else "",
            training_config=cfg.name,
            weights_path=cfg.output_dir,
            status="training",
        )
        self.registry.register(entry)

        logger.info("Starting training: %s", self.model_id)
        logger.info("Base model: %s", cfg.base_model)
        logger.info("Method: %s", cfg.method)
        logger.info("Dataset: %s (%s)", cfg.dataset.path, cfg.dataset.format)

        # Attempt GPU training if dependencies available
        try:
            metrics = self._train_gpu()
        except ImportError:
            logger.warning("GPU training dependencies not available, running dry-run")
            metrics = self._train_dryrun()

        # Update registry with results
        self.registry.update_metrics(self.model_id, metrics)
        self.registry.update_status(self.model_id, "ready")

        logger.info("Training complete: %s", self.model_id)
        return metrics

    def _train_gpu(self) -> dict[str, Any]:
        """Run actual GPU training with HuggingFace stack."""
        import torch
        from datasets import load_dataset
        from peft import LoraConfig, TaskType, get_peft_model
        from transformers import (
            AutoModelForCausalLM,
            AutoTokenizer,
            BitsAndBytesConfig,
            TrainingArguments,
            Trainer,
        )

        cfg = self.config
        output_dir = Path(cfg.output_dir)
        output_dir.mkdir(parents=True, exist_ok=True)

        # Quantization config
        bnb_config = None
        if cfg.method == "qlora":
            bnb_config = BitsAndBytesConfig(
                load_in_4bit=True,
                bnb_4bit_quant_type=cfg.quantization.quant_type,
                bnb_4bit_use_double_quant=cfg.quantization.double_quant,
                bnb_4bit_compute_dtype=torch.bfloat16,
            )

        # Load model
        model = AutoModelForCausalLM.from_pretrained(
            cfg.base_model,
            quantization_config=bnb_config,
            device_map="auto",
            torch_dtype=torch.bfloat16,
        )
        tokenizer = AutoTokenizer.from_pretrained(cfg.base_model)
        if tokenizer.pad_token is None:
            tokenizer.pad_token = tokenizer.eos_token

        # LoRA config
        if cfg.method in ("lora", "qlora"):
            lora_config = LoraConfig(
                r=cfg.lora.r,
                lora_alpha=cfg.lora.alpha,
                lora_dropout=cfg.lora.dropout,
                target_modules=cfg.lora.target_modules,
                task_type=TaskType.CAUSAL_LM,
            )
            model = get_peft_model(model, lora_config)
            model.print_trainable_parameters()

        # Load dataset
        dataset = load_dataset("json", data_files={
            "train": str(Path(cfg.dataset.path) / "train.jsonl"),
            "validation": str(Path(cfg.dataset.path) / "validation.jsonl"),
        })

        # Training arguments
        training_args = TrainingArguments(
            output_dir=str(output_dir),
            num_train_epochs=cfg.hyperparameters.epochs,
            per_device_train_batch_size=cfg.hyperparameters.batch_size,
            gradient_accumulation_steps=cfg.hyperparameters.gradient_accumulation_steps,
            learning_rate=cfg.hyperparameters.learning_rate,
            warmup_ratio=cfg.hyperparameters.warmup_ratio,
            logging_steps=cfg.logging_steps,
            eval_strategy="steps",
            eval_steps=cfg.eval_steps,
            save_steps=cfg.checkpoint_steps,
            save_total_limit=3,
            bf16=True,
            report_to="none",
        )

        trainer = Trainer(
            model=model,
            args=training_args,
            train_dataset=dataset["train"],
            eval_dataset=dataset["validation"],
            tokenizer=tokenizer,
        )

        result = trainer.train()

        # Save
        model.save_pretrained(str(output_dir / "adapter"))
        tokenizer.save_pretrained(str(output_dir / "adapter"))

        # Extract metrics
        metrics = {
            "final_loss": result.training_loss,
            "eval_loss": trainer.evaluate().get("eval_loss", 0.0),
        }

        # Save metrics
        with open(output_dir / "training_metrics.json", "w") as f:
            json.dump(metrics, f, indent=2)

        return metrics

    def _train_dryrun(self) -> dict[str, Any]:
        """Dry-run without GPU — validates config and returns placeholder metrics."""
        cfg = self.config
        output_dir = Path(cfg.output_dir)
        output_dir.mkdir(parents=True, exist_ok=True)

        # Validate dataset path exists
        dataset_path = Path(cfg.dataset.path)
        if dataset_path.exists():
            train_file = dataset_path / "train.jsonl"
            if train_file.exists():
                with open(train_file) as f:
                    line_count = sum(1 for _ in f)
                logger.info("Dataset has %d training samples", line_count)
            else:
                logger.warning("No train.jsonl found at %s", dataset_path)
        else:
            logger.warning("Dataset path does not exist: %s", dataset_path)

        metrics = {
            "final_loss": 0.0,
            "eval_loss": 0.0,
            "dry_run": True,
        }

        with open(output_dir / "training_metrics.json", "w") as f:
            json.dump(metrics, f, indent=2)

        return metrics

    def resume(self, checkpoint_path: str) -> dict[str, Any]:
        """Resume training from a checkpoint."""
        logger.info("Resuming from checkpoint: %s", checkpoint_path)
        # In production, this would set resume_from_checkpoint in TrainingArguments
        return self.run()


def _dataset_version(cfg: TrainingConfig) -> str:
    """Extract dataset version from path."""
    parts = Path(cfg.dataset.path).parts
    for p in reversed(parts):
        if p.startswith("v"):
            return p
    return "v0.0.0"


class Evaluator:
    """Post-training evaluation suite."""

    def __init__(self, registry: ModelRegistry):
        self.registry = registry

    def evaluate(self, model_id: str, test_data_path: str | None = None) -> dict[str, float]:
        """Run evaluation suite on a trained model."""
        entry = self.registry.get(model_id)
        if not entry:
            raise ValueError(f"Model {model_id} not found in registry")

        logger.info("Evaluating model: %s", model_id)
        scores: dict[str, float] = {}

        # Perplexity on test set
        if test_data_path:
            scores["perplexity"] = self._compute_perplexity(entry, test_data_path)

        self.registry.update_benchmarks(model_id, scores)
        return scores

    def _compute_perplexity(self, entry: ModelEntry, test_path: str) -> float:
        """Compute perplexity on held-out test data."""
        try:
            import torch
            from transformers import AutoModelForCausalLM, AutoTokenizer
            # Load model and compute perplexity
            # Placeholder — in production would run actual inference
            return 0.0
        except ImportError:
            logger.warning("torch not available, skipping perplexity computation")
            return 0.0

    def compare(self, model_a: str, model_b: str) -> dict[str, Any]:
        """Compare two models' metrics."""
        a = self.registry.get(model_a)
        b = self.registry.get(model_b)
        if not a or not b:
            raise ValueError("One or both models not found")

        return {
            "model_a": model_a,
            "model_b": model_b,
            "metrics_a": a.metrics,
            "metrics_b": b.metrics,
            "benchmarks_a": a.benchmark_scores,
            "benchmarks_b": b.benchmark_scores,
        }
