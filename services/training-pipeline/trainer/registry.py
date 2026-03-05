"""Local model registry for tracking trained models."""

from __future__ import annotations

import json
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


@dataclass
class ModelEntry:
    model_id: str
    base_model: str
    dataset_version: str
    method: str
    domain_focus: str = ""
    training_config: str = ""
    metrics: dict[str, float] = field(default_factory=dict)
    benchmark_scores: dict[str, float] = field(default_factory=dict)
    weights_path: str = ""
    created_at: str = ""
    status: str = "training"  # training, ready, serving, archived

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> ModelEntry:
        return cls(**{k: v for k, v in data.items() if k in cls.__dataclass_fields__})


class ModelRegistry:
    """JSON-file-backed registry of trained models."""

    def __init__(self, path: str | Path = "model_registry.json"):
        self.path = Path(path)
        self._entries: dict[str, ModelEntry] = {}
        self._load()

    def _load(self) -> None:
        if self.path.exists():
            with open(self.path) as f:
                data = json.load(f)
            for entry_data in data.get("models", []):
                entry = ModelEntry.from_dict(entry_data)
                self._entries[entry.model_id] = entry

    def _save(self) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        data = {"models": [e.to_dict() for e in self._entries.values()]}
        with open(self.path, "w") as f:
            json.dump(data, f, indent=2)

    def register(self, entry: ModelEntry) -> None:
        """Register a new model entry."""
        if not entry.created_at:
            entry.created_at = datetime.now(timezone.utc).isoformat()
        self._entries[entry.model_id] = entry
        self._save()

    def get(self, model_id: str) -> ModelEntry | None:
        return self._entries.get(model_id)

    def list_models(self, status: str | None = None) -> list[ModelEntry]:
        entries = list(self._entries.values())
        if status:
            entries = [e for e in entries if e.status == status]
        return sorted(entries, key=lambda e: e.created_at, reverse=True)

    def update_status(self, model_id: str, status: str) -> bool:
        if model_id not in self._entries:
            return False
        self._entries[model_id].status = status
        self._save()
        return True

    def update_metrics(self, model_id: str, metrics: dict[str, float]) -> bool:
        if model_id not in self._entries:
            return False
        self._entries[model_id].metrics.update(metrics)
        self._save()
        return True

    def update_benchmarks(self, model_id: str, scores: dict[str, float]) -> bool:
        if model_id not in self._entries:
            return False
        self._entries[model_id].benchmark_scores.update(scores)
        self._save()
        return True
