"""Mirror's blending and decay maths (03 §4.1, FR-068, P5)."""

from __future__ import annotations

import math

ALPHA_MIN = 0.15  # stated preference never drops below 15% weight (product rule P5)
ALPHA_MAX = 0.85
DEFAULT_HALF_LIFE_DAYS = 90.0


def alpha(weights: list[float], features: list[float], personalisation_enabled: bool = True) -> float:
    """alpha = sigmoid(w . x), clamped to [0.15, 0.85]; frozen at 0.15 when personalisation is off."""
    if not personalisation_enabled:
        return ALPHA_MIN
    if len(weights) != len(features):
        raise ValueError("weights and features differ in length")
    z = sum(w * x for w, x in zip(weights, features))
    # Numerically stable sigmoid (property tests found overflow for large |z|).
    raw = 1.0 / (1.0 + math.exp(-z)) if z >= 0 else math.exp(z) / (1.0 + math.exp(z))
    return min(ALPHA_MAX, max(ALPHA_MIN, raw))


def effective_vector(revealed: list[float], stated_projected: list[float], a: float) -> list[float]:
    if not ALPHA_MIN <= a <= ALPHA_MAX:
        raise ValueError("alpha outside [0.15, 0.85]")
    if len(revealed) != len(stated_projected):
        raise ValueError("dimension mismatch")
    return [a * r + (1.0 - a) * s for r, s in zip(revealed, stated_projected)]


def decay_weight(age_days: float, half_life_days: float = DEFAULT_HALF_LIFE_DAYS) -> float:
    if age_days < 0:
        raise ValueError("age must be non-negative")
    return 0.5 ** (age_days / half_life_days)


def divergence(stated_projected: list[float], revealed: list[float]) -> float:
    """1 - cosine similarity. Used as a signal, never 'corrected' away."""
    dot = sum(a * b for a, b in zip(stated_projected, revealed))
    na = math.sqrt(sum(a * a for a in stated_projected))
    nb = math.sqrt(sum(b * b for b in revealed))
    if na == 0 or nb == 0:
        return 1.0
    return 1.0 - dot / (na * nb)
