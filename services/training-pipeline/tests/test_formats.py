"""Tests for format mappings."""

import pytest

from transforms.formats import Sample, to_training_format


def _sample(sample_type: str, content: str = "", **kwargs) -> Sample:
    defaults = {
        "id": "test-1",
        "content": content or f"Test content for {sample_type}",
        "sample_type": sample_type,
        "domain": "science",
        "quality_tier": "gold",
    }
    defaults.update(kwargs)
    return Sample(**defaults)


class TestChatFormat:
    def test_qa_with_markers(self):
        s = _sample("q_and_a", "Q: What is gravity?\nA: Gravity is the fundamental force of attraction between masses.")
        result = to_training_format(s)
        assert "messages" in result
        msgs = result["messages"]
        assert msgs[0]["role"] == "system"
        assert msgs[1]["role"] == "user"
        assert "gravity" in msgs[1]["content"].lower()
        assert msgs[2]["role"] == "assistant"
        assert "force" in msgs[2]["content"].lower()

    def test_explanation_format(self):
        s = _sample("explanation", "Photosynthesis overview\n\nPhotosynthesis converts light energy into chemical energy stored in glucose molecules.")
        result = to_training_format(s)
        assert "messages" in result
        assert any(m["role"] == "assistant" for m in result["messages"])

    def test_tutorial_format(self):
        s = _sample("tutorial", "How to bake bread\n\nFirst, mix flour and water. Then knead the dough for 10 minutes.")
        result = to_training_format(s)
        assert "messages" in result

    def test_troubleshoot_format(self):
        s = _sample("troubleshoot", "Q: Why won't my code compile?\nA: You likely have a missing import statement. Check your imports.")
        result = to_training_format(s)
        assert result["messages"][-1]["role"] == "assistant"


class TestConversationFormat:
    def test_discussion(self):
        s = _sample("discussion", "Speaker 1: I think X is true.\nSpeaker 2: Actually, evidence shows Y.")
        result = to_training_format(s)
        assert "messages" in result
        assert len(result["messages"]) >= 3  # system + at least 2 turns

    def test_debate(self):
        s = _sample("debate", "First point about topic.\n\nCounterpoint with different view.\n\nRebuttal and synthesis.")
        result = to_training_format(s)
        assert "messages" in result
        # Last message should be assistant
        assert result["messages"][-1]["role"] == "assistant"


class TestCompletionFormat:
    def test_narrative(self):
        s = _sample("narrative", "Once upon a time in a galaxy far away, things happened.")
        result = to_training_format(s)
        assert "text" in result
        assert "galaxy" in result["text"]

    def test_creative(self):
        s = _sample("creative", "The wind whispered through ancient trees.")
        result = to_training_format(s)
        assert "text" in result

    def test_opinion(self):
        s = _sample("opinion", "I believe that open source is crucial for innovation.")
        result = to_training_format(s)
        assert "text" in result

    def test_domain_context_prepended(self):
        s = _sample("narrative", "Story content.", domain="literature")
        result = to_training_format(s, domain_context=True)
        assert "[literature]" in result["text"]

    def test_no_domain_context(self):
        s = _sample("narrative", "Story content.", domain="literature")
        result = to_training_format(s, domain_context=False)
        assert "[literature]" not in result["text"]


class TestDPOFormat:
    def test_correction(self):
        s = _sample("correction", "Original: The earth is flat.\nCorrected: The earth is roughly spherical.")
        result = to_training_format(s)
        assert "chosen" in result
        assert "rejected" in result
        assert "spherical" in result["chosen"]
        assert "flat" in result["rejected"]

    def test_review_fallback(self):
        s = _sample("review", "This is a detailed review of the paper.")
        result = to_training_format(s)
        assert "chosen" in result

    def test_annotation_uses_chat(self):
        s = _sample("annotation", "Q: Label this text.\nA: This is a factual claim about biology.")
        result = to_training_format(s)
        assert "messages" in result


class TestUnknownType:
    def test_unknown_falls_to_completion(self):
        s = _sample("unknown_type", "Some random content here.")
        result = to_training_format(s)
        assert "text" in result
