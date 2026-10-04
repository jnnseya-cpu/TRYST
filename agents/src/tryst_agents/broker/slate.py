"""Stages 3-4: constraints and slate assembly (03 §5.1, §5.4; FR-012, FR-013)."""

from __future__ import annotations

import math
from dataclasses import dataclass

SLATE_SIZE = 18
EXPLORATION_FLOOR = 0.15
DEFAULT_INBOUND_CAP = 12  # intents per 24 h; member-adjustable 4-40
INBOUND_CAP_RANGE = (4, 40)


@dataclass(frozen=True)
class Candidate:
    id: str
    score: float
    inbound_last_24h: int = 0
    inbound_cap: int = DEFAULT_INBOUND_CAP


def clamp_cap(requested: int) -> int:
    lo, hi = INBOUND_CAP_RANGE
    return max(lo, min(hi, requested))


def assemble_slate(
    ranked: list[Candidate],
    exploration: list[Candidate],
    size: int = SLATE_SIZE,
    explore_frac: float = EXPLORATION_FLOOR,
) -> list[tuple[Candidate, bool]]:
    """Return up to `size` (candidate, is_exploration) pairs.

    At least ceil(size * explore_frac) slots are reserved for exploration when
    exploration candidates exist. Candidates at their inbound cap are skipped so a
    slate can never push anyone past the cap (S4 protection).
    """
    if explore_frac < EXPLORATION_FLOOR:
        raise ValueError("exploration floor is 15% and cannot be lowered")

    def has_room(c: Candidate) -> bool:
        return c.inbound_last_24h < clamp_cap(c.inbound_cap)

    seen: set[str] = set()
    explore_quota = math.ceil(size * explore_frac)
    picks: list[tuple[Candidate, bool]] = []

    for c in exploration:
        if len(picks) >= explore_quota:
            break
        if c.id not in seen and has_room(c):
            picks.append((c, True))
            seen.add(c.id)

    for c in sorted(ranked, key=lambda c: c.score, reverse=True):
        if len(picks) >= size:
            break
        if c.id not in seen and has_room(c):
            picks.append((c, False))
            seen.add(c.id)
    return picks
