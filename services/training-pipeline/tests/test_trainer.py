"""Tests for training config, registry, and runner."""

import json
import os
import tempfile
from pathlib import Path

import pytest

from trainer.config import TrainingConfig
from trainer.registry import ModelEntry, ModelRegistry
from trainer.runner import TrainingRunner, _dataset_version


class TestTrainingConfig:
    def test_from_yaml(self):
        cfg = TrainingConfig.from_yaml("configs/technical-v1.yaml")
        assert cfg.name == "zerone-technical-v1"
        assert cfg.base_model == "meta-llama/Llama-3.1-8B-Instruct"
        assert cfg.method == "qlora"
        assert cfg.dataset.format == "chat"
        assert cfg.hyperparameters.learning_rate == 2e-4
        assert cfg.hyperparameters.epochs == 3
        assert cfg.lora.r == 64
        assert cfg.lora.alpha == 128
        assert "q_proj" in cfg.lora.target_modules
        assert cfg.quantization.bits == 4
        assert cfg.quantization.quant_type == "nf4"

    def test_defaults(self):
        cfg = TrainingConfig()
        assert cfg.method == "qlora"
        assert cfg.hyperparameters.batch_size == 4
        assert cfg.lora.dropout == 0.05

    def test_custom_yaml(self, tmp_path):
        yaml_content = """
name: "test-model"
base_model: "test/model"
method: "lora"
dataset:
  path: "/tmp/data"
  format: "completion"
hyperparameters:
  learning_rate: 1e-5
  epochs: 1
lora:
  r: 16
  alpha: 32
"""
        p = tmp_path / "config.yaml"
        p.write_text(yaml_content)
        cfg = TrainingConfig.from_yaml(p)
        assert cfg.name == "test-model"
        assert cfg.method == "lora"
        assert cfg.hyperparameters.learning_rate == 1e-5
        assert cfg.lora.r == 16


class TestModelRegistry:
    def test_register_and_get(self, tmp_path):
        reg = ModelRegistry(tmp_path / "registry.json")
        entry = ModelEntry(
            model_id="test-v1",
            base_model="test/model",
            dataset_version="v1.0.0",
            method="qlora",
        )
        reg.register(entry)
        result = reg.get("test-v1")
        assert result is not None
        assert result.model_id == "test-v1"
        assert result.created_at != ""

    def test_list_models(self, tmp_path):
        reg = ModelRegistry(tmp_path / "registry.json")
        for i in range(3):
            reg.register(ModelEntry(
                model_id=f"model-{i}",
                base_model="test",
                dataset_version="v1",
                method="lora",
                status="ready" if i < 2 else "training",
            ))
        assert len(reg.list_models()) == 3
        assert len(reg.list_models(status="ready")) == 2
        assert len(reg.list_models(status="training")) == 1

    def test_update_status(self, tmp_path):
        reg = ModelRegistry(tmp_path / "registry.json")
        reg.register(ModelEntry(
            model_id="m1", base_model="t", dataset_version="v1", method="lora",
        ))
        assert reg.update_status("m1", "serving")
        assert reg.get("m1").status == "serving"
        assert not reg.update_status("nonexistent", "ready")

    def test_update_metrics(self, tmp_path):
        reg = ModelRegistry(tmp_path / "registry.json")
        reg.register(ModelEntry(
            model_id="m1", base_model="t", dataset_version="v1", method="lora",
        ))
        reg.update_metrics("m1", {"final_loss": 0.42, "eval_loss": 0.45})
        entry = reg.get("m1")
        assert entry.metrics["final_loss"] == 0.42

    def test_persistence(self, tmp_path):
        path = tmp_path / "registry.json"
        reg1 = ModelRegistry(path)
        reg1.register(ModelEntry(
            model_id="persist-test", base_model="t", dataset_version="v1", method="lora",
        ))

        # Load from same file
        reg2 = ModelRegistry(path)
        assert reg2.get("persist-test") is not None

    def test_update_benchmarks(self, tmp_path):
        reg = ModelRegistry(tmp_path / "registry.json")
        reg.register(ModelEntry(
            model_id="m1", base_model="t", dataset_version="v1", method="lora",
        ))
        reg.update_benchmarks("m1", {"mmlu": 0.65, "hellaswag": 0.72})
        entry = reg.get("m1")
        assert entry.benchmark_scores["mmlu"] == 0.65


class TestTrainingRunner:
    def test_dryrun(self, tmp_path):
        cfg = TrainingConfig(
            name="test-dry",
            method="qlora",
            output_dir=str(tmp_path / "output"),
        )
        cfg.dataset.path = str(tmp_path / "data")
        reg = ModelRegistry(tmp_path / "registry.json")
        runner = TrainingRunner(cfg, reg)

        metrics = runner.run()
        assert "dry_run" in metrics
        assert metrics["dry_run"] is True

        # Check model registered
        entry = reg.get(runner.model_id)
        assert entry is not None
        assert entry.status == "ready"

    def test_dataset_version_extraction(self):
        cfg = TrainingConfig()
        cfg.dataset.path = "/data/training/v1.2.0/"
        assert _dataset_version(cfg) == "v1.2.0"

        cfg.dataset.path = "/data/training/latest/"
        assert _dataset_version(cfg) == "v0.0.0"


class TestModelEntry:
    def test_serialization(self):
        entry = ModelEntry(
            model_id="test-1",
            base_model="llama",
            dataset_version="v1",
            method="qlora",
            metrics={"loss": 0.5},
        )
        d = entry.to_dict()
        assert d["model_id"] == "test-1"
        assert d["metrics"]["loss"] == 0.5

        restored = ModelEntry.from_dict(d)
        assert restored.model_id == "test-1"
        assert restored.metrics["loss"] == 0.5
