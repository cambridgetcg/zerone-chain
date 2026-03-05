"""Training configuration loader."""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import yaml


@dataclass
class LoRAConfig:
    r: int = 64
    alpha: int = 128
    dropout: float = 0.05
    target_modules: list[str] = field(default_factory=lambda: [
        "q_proj", "k_proj", "v_proj", "o_proj",
        "gate_proj", "up_proj", "down_proj",
    ])


@dataclass
class QuantConfig:
    bits: int = 4
    quant_type: str = "nf4"
    double_quant: bool = True


@dataclass
class DatasetConfig:
    path: str = ""
    format: str = "chat"


@dataclass
class HyperParams:
    learning_rate: float = 2e-4
    batch_size: int = 4
    gradient_accumulation_steps: int = 8
    epochs: int = 3
    max_seq_length: int = 4096
    warmup_ratio: float = 0.03


@dataclass
class TrainingConfig:
    name: str = ""
    base_model: str = "meta-llama/Llama-3.1-8B-Instruct"
    method: str = "qlora"
    dataset: DatasetConfig = field(default_factory=DatasetConfig)
    hyperparameters: HyperParams = field(default_factory=HyperParams)
    lora: LoRAConfig = field(default_factory=LoRAConfig)
    quantization: QuantConfig = field(default_factory=QuantConfig)
    output_dir: str = "/models/output/"
    checkpoint_steps: int = 500
    eval_steps: int = 250
    logging_steps: int = 10

    @classmethod
    def from_yaml(cls, path: str | Path) -> TrainingConfig:
        """Load config from a YAML file."""
        with open(path) as f:
            data = yaml.safe_load(f)
        return cls._from_dict(data)

    @classmethod
    def _from_dict(cls, data: dict[str, Any]) -> TrainingConfig:
        cfg = cls()
        cfg.name = data.get("name", "")
        cfg.base_model = data.get("base_model", cfg.base_model)
        cfg.method = data.get("method", cfg.method)
        cfg.output_dir = data.get("output_dir", cfg.output_dir)
        cfg.checkpoint_steps = data.get("checkpoint_steps", cfg.checkpoint_steps)
        cfg.eval_steps = data.get("eval_steps", cfg.eval_steps)
        cfg.logging_steps = data.get("logging_steps", cfg.logging_steps)

        if "dataset" in data:
            d = data["dataset"]
            cfg.dataset = DatasetConfig(
                path=d.get("path", ""),
                format=d.get("format", "chat"),
            )

        if "hyperparameters" in data:
            h = data["hyperparameters"]
            cfg.hyperparameters = HyperParams(
                learning_rate=float(h.get("learning_rate", 2e-4)),
                batch_size=h.get("batch_size", 4),
                gradient_accumulation_steps=h.get("gradient_accumulation_steps", 8),
                epochs=h.get("epochs", 3),
                max_seq_length=h.get("max_seq_length", 4096),
                warmup_ratio=h.get("warmup_ratio", 0.03),
            )

        if "lora" in data:
            lo = data["lora"]
            cfg.lora = LoRAConfig(
                r=lo.get("r", 64),
                alpha=lo.get("alpha", 128),
                dropout=lo.get("dropout", 0.05),
                target_modules=lo.get("target_modules", cfg.lora.target_modules),
            )

        if "quantization" in data:
            q = data["quantization"]
            cfg.quantization = QuantConfig(
                bits=q.get("bits", 4),
                quant_type=q.get("quant_type", "nf4"),
                double_quant=q.get("double_quant", True),
            )

        return cfg
