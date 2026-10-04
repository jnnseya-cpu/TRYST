"""P1 launch ranker (03 §5.2) and multi-person max-min scoring (03 §5.3)."""

from __future__ import annotations

import math

P1_WEIGHTS = {
    "reciprocal_attraction": 0.24,
    "intent_fit": 0.20,
    "discretion_fit": 0.15,
    "communication_fit": 0.12,
    "availability_logistics": 0.10,
    "reliability": 0.08,
    "novelty": 0.06,
    "model_confidence": 0.05,
}


def p1_score(features: dict[str, float], risk_penalty: float = 0.0) -> float:
    """Weighted baseline. Features are in [0, 1]; missing features count as 0."""
    for name, value in features.items():
        if name not in P1_WEIGHTS:
            raise KeyError(f"unknown feature {name!r}")
        if not 0.0 <= value <= 1.0:
            raise ValueError(f"{name} must be in [0, 1]")
    base = sum(w * features.get(k, 0.0) for k, w in P1_WEIGHTS.items())
    return base - max(0.0, risk_penalty)


def soft_min(edge_scores: list[float], tau: float = 0.1) -> float:
    """Smooth max-min over required edges: one strong edge cannot mask a weak one."""
    if not edge_scores:
        raise ValueError("at least one required edge")
    return -tau * math.log(sum(math.exp(-s / tau) for s in edge_scores) / len(edge_scores))
