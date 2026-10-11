"""Matchable entity as seen by Broker's Stage 0 gates (02 §2, §4.3)."""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import IntEnum


class Tier(IntEnum):
    V0 = 0
    V1 = 1
    V2 = 2
    V3 = 3


SOLO_SEGMENTS = {"S1", "S2", "S4", "S6"}
COUPLE_SEGMENTS = {"S3", "S5"}


@dataclass(frozen=True)
class Entity:
    id: str
    entity_type: str  # "solo" | "couple"
    segment: str
    tier: Tier
    min_counterparty_tier: Tier = Tier.V2
    modes: frozenset[str] = frozenset()  # intent_shape values
    hard_limits: frozenset[str] = frozenset()
    desire_tags: dict[str, str] = field(default_factory=dict)  # tag -> yes|curious|no
    location_cell: str = ""  # geohash-5
    zones: frozenset[str] = frozenset()  # geohash-6 exclusion cells (home/work)
    jurisdiction: str = "GB"
    visibility: str = "public"  # public|matched|incognito
    couple_valid: bool = True  # VC: both partners V2 + distinct-device co-sign
    safety_state: str = "normal"
    blocked: frozenset[str] = frozenset()
    excluded: frozenset[str] = frozenset()  # ExclusionRing partners
    outbound_intents: frozenset[str] = frozenset()
    # Secret-keeping (FR-106, FR-107, D-45). Accounts behind this profile: one for a solo
    # profile, two for a couple. A person may hold a private solo profile and a couple
    # profile; the gate below keeps them apart from each other and from the partner.
    owners: frozenset[str] = frozenset()
    # Accounts these owners must never see or be seen by: their couple partners on TRYST
    # and partner-shield matches (PSI on the partner's phone/email, heuristics). Silent.
    shielded_accounts: frozenset[str] = frozenset()

    def __hash__(self) -> int:  # dict field makes the default hash unusable
        return hash(self.id)
