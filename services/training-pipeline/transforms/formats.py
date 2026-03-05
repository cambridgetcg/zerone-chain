"""Format mappings from SampleType to HuggingFace training formats."""

from __future__ import annotations

import json
import re
from dataclasses import dataclass
from typing import Any


@dataclass
class Sample:
    """A training data sample from the staging database."""

    id: str
    content: str
    sample_type: str
    domain: str
    quality_tier: str
    quality_score: int = 0
    novelty_score: int = 0
    source_uri: str = ""
    source_platform: str = ""
    original_author: str = ""
    language: str = "en"
    tags: list[str] | None = None
    thread_id: str = ""
    parent_sample_id: str = ""
    thread_position: int = 0


# ── Format types ──────────────────────────────────────────────────────────────

CHAT_TYPES = {"q_and_a", "explanation", "tutorial", "troubleshoot"}
CONVERSATION_TYPES = {"discussion", "debate"}
COMPLETION_TYPES = {"narrative", "creative", "opinion"}
DPO_TYPES = {"correction", "review"}
ANNOTATION_TYPES = {"annotation"}


def to_training_format(sample: Sample, domain_context: bool = True) -> dict[str, Any]:
    """Convert a sample to the appropriate training format based on sample_type."""
    st = sample.sample_type.lower()

    if st in CHAT_TYPES:
        return _to_chat(sample, domain_context)
    elif st in CONVERSATION_TYPES:
        return _to_conversation(sample, domain_context)
    elif st in COMPLETION_TYPES:
        return _to_completion(sample, domain_context)
    elif st in DPO_TYPES:
        return _to_dpo(sample)
    elif st in ANNOTATION_TYPES:
        return _to_chat(sample, domain_context)  # annotation → chat format
    else:
        # Fallback: completion format
        return _to_completion(sample, domain_context)


def _system_prompt(sample: Sample) -> str:
    """Build a system prompt with optional domain context."""
    parts = [f"You are a knowledgeable assistant specializing in {sample.domain}."]
    if sample.quality_tier == "gold":
        parts.append("Provide detailed, high-quality responses.")
    return " ".join(parts)


def _to_chat(sample: Sample, domain_context: bool) -> dict[str, Any]:
    """Convert to chat/instruction format."""
    messages = []
    if domain_context:
        messages.append({"role": "system", "content": _system_prompt(sample)})

    # Try to split content into Q/A parts
    parts = _split_qa(sample.content)
    if parts:
        messages.append({"role": "user", "content": parts[0]})
        messages.append({"role": "assistant", "content": parts[1]})
    else:
        # Single content block — frame as instruction
        messages.append({"role": "user", "content": f"Explain: {sample.domain}"})
        messages.append({"role": "assistant", "content": sample.content})

    return {"messages": messages}


def _to_conversation(sample: Sample, domain_context: bool) -> dict[str, Any]:
    """Convert to multi-turn conversation format."""
    messages = []
    if domain_context:
        messages.append({"role": "system", "content": _system_prompt(sample)})

    # Split into turns (paragraphs or marked sections)
    turns = _split_turns(sample.content)
    for i, turn in enumerate(turns):
        role = "user" if i % 2 == 0 else "assistant"
        messages.append({"role": role, "content": turn})

    # Ensure ends with assistant turn
    if messages and messages[-1]["role"] == "user":
        messages.append({"role": "assistant", "content": "I see your point."})

    return {"messages": messages}


def _to_completion(sample: Sample, domain_context: bool) -> dict[str, Any]:
    """Convert to completion/pretraining format."""
    text = sample.content
    if domain_context and sample.domain:
        text = f"[{sample.domain}] {text}"
    return {"text": text}


def _to_dpo(sample: Sample) -> dict[str, Any]:
    """Convert to DPO/preference format."""
    parts = _split_correction(sample.content)
    if parts:
        return {"prompt": parts[0], "chosen": parts[2], "rejected": parts[1]}
    # Fallback: treat full content as chosen, empty as rejected
    return {"prompt": "", "chosen": sample.content, "rejected": ""}


def _split_qa(content: str) -> tuple[str, str] | None:
    """Try to split content into question and answer parts."""
    # Look for Q:/A: or Question:/Answer: markers
    patterns = [
        r"(?:^|\n)Q:\s*(.*?)(?:\n)A:\s*(.*)",
        r"(?:^|\n)Question:\s*(.*?)(?:\n)Answer:\s*(.*)",
        r"(?:^|\n)\*\*Q\*\*:\s*(.*?)(?:\n)\*\*A\*\*:\s*(.*)",
    ]
    for pat in patterns:
        m = re.search(pat, content, re.DOTALL | re.IGNORECASE)
        if m:
            return m.group(1).strip(), m.group(2).strip()

    # Try splitting on double newline (first para = question, rest = answer)
    parts = content.split("\n\n", 1)
    if len(parts) == 2 and len(parts[0]) < len(parts[1]):
        return parts[0].strip(), parts[1].strip()

    return None


def _split_turns(content: str) -> list[str]:
    """Split content into conversation turns."""
    # Try speaker markers: "Speaker 1:", "A:", "B:", etc.
    turn_pattern = r"(?:^|\n)(?:Speaker\s*\d+|[A-Z]):\s*"
    parts = re.split(turn_pattern, content)
    parts = [p.strip() for p in parts if p.strip()]

    if len(parts) >= 2:
        return parts

    # Fallback: split on double newlines
    parts = [p.strip() for p in content.split("\n\n") if p.strip()]
    return parts if len(parts) >= 2 else [content]


def _split_correction(content: str) -> tuple[str, str, str] | None:
    """Try to split correction content into prompt/original/corrected."""
    patterns = [
        r"(?:Original|Wrong|Incorrect):\s*(.*?)(?:\n)(?:Corrected|Right|Correct):\s*(.*)",
        r"(?:Claim|Statement):\s*(.*?)(?:\n)(?:Correction|Fix):\s*(.*)",
    ]
    for pat in patterns:
        m = re.search(pat, content, re.DOTALL | re.IGNORECASE)
        if m:
            prompt = "Is the following statement correct?"
            return prompt, m.group(1).strip(), m.group(2).strip()
    return None


def format_jsonl(records: list[dict[str, Any]]) -> str:
    """Serialize records to JSONL string."""
    return "\n".join(json.dumps(r, ensure_ascii=False) for r in records) + "\n"
