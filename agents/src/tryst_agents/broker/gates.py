"""Stage 0 hard gates (03 §5.1, FR-014).

Pure, non-scored, non-bypassable. Every gate except visibility is symmetric:
if A may see B then B may see A. Any violation surfacing a candidate is a P0
incident, so these functions are property-tested (tests/test_gates.py).
"""

from __future__ import annotations

from .entities import COUPLE_SEGMENTS, SOLO_SEGMENTS, Entity, Tier

ALLOWED_JURISDICTIONS = frozenset({"GB", "IE"})  # launch set (01 §11); config in production

# Which entity pairings each intent shape allows (02 §2.3).
_SOLO_SOLO = {"one_off", "recurring", "ongoing", "exploratory"}


def _shape_pair_ok(shape: str, a: Entity, b: Entity) -> bool:
    if shape in _SOLO_SOLO - {"exploratory"}:
        return a.entity_type == "solo" and b.entity_type == "solo"
    if shape == "third":
        return {a.segment, b.segment} == {"S3", "S4"}
    if shape == "couple":
        return a.segment == "S5" and b.segment == "S5"
    return shape == "exploratory"


def structure_compatible(a: Entity, b: Entity) -> bool:
    shared = a.modes & b.modes
    return any(_shape_pair_ok(s, a, b) for s in shared)


# Hard limits that constrain the counterpart's shape. Others (no_overnight, ...) are
# logistics limits enforced in the handshake, and only collide via desire tags.
def _limit_excludes(limit: str, other: Entity) -> bool:
    if limit == "no_couples":
        return other.entity_type == "couple"
    if limit == "no_solo_third":
        return other.segment == "S4"
    return False


def hard_limits_clear(a: Entity, b: Entity) -> bool:
    if any(_limit_excludes(lim, b) for lim in a.hard_limits):
        return False
    if any(_limit_excludes(lim, a) for lim in b.hard_limits):
        return False
    for tag, level in a.desire_tags.items():
        other = b.desire_tags.get(tag)
        if {level, other} == {"no", "yes"}:
            return False
    return True


def tier_parity(a: Entity, b: Entity) -> bool:
    return (
        a.tier >= Tier.V2
        and b.tier >= Tier.V2
        and a.tier >= b.min_counterparty_tier
        and b.tier >= a.min_counterparty_tier
    )


def _in_zone(entity_cell: str, zones: frozenset[str]) -> bool:
    # Location is geohash-5; zones are geohash-6. Conservative: any zone inside the cell excludes.
    return bool(entity_cell) and any(z.startswith(entity_cell) for z in zones)


def geofence_clear(a: Entity, b: Entity) -> bool:
    return not _in_zone(b.location_cell, a.zones) and not _in_zone(a.location_cell, b.zones)


def valid_entity(e: Entity) -> bool:
    if e.entity_type == "couple":
        return e.couple_valid and e.segment in COUPLE_SEGMENTS
    return e.entity_type == "solo" and e.segment in SOLO_SEGMENTS


def mutual_eligible(a: Entity, b: Entity) -> tuple[bool, str]:
    """Symmetric Stage 0 gates. Returns (eligible, first failing gate)."""
    if a.id == b.id:
        return False, "self"
    checks = (
        ("entity_valid", valid_entity(a) and valid_entity(b)),
        ("tier", tier_parity(a, b)),
        ("hard_limits", hard_limits_clear(a, b)),
        ("structure", structure_compatible(a, b)),
        ("exclusion_ring", b.id not in a.excluded and a.id not in b.excluded),
        ("geofence", geofence_clear(a, b)),
        ("block", b.id not in a.blocked and a.id not in b.blocked),
        ("safety", a.safety_state == "normal" and b.safety_state == "normal"),
        (
            "jurisdiction",
            a.jurisdiction in ALLOWED_JURISDICTIONS and b.jurisdiction in ALLOWED_JURISDICTIONS,
        ),
    )
    for name, ok in checks:
        if not ok:
            return False, name
    return True, ""


def visible_to(viewer: Entity, candidate: Entity) -> bool:
    """Asymmetric visibility (FR-009): incognito members appear only to people they've sent an intent to."""
    if candidate.visibility == "incognito":
        return viewer.id in candidate.outbound_intents
    return True


def eligible_candidates(viewer: Entity, pool: list[Entity]) -> list[Entity]:
    return [c for c in pool if mutual_eligible(viewer, c)[0] and visible_to(viewer, c)]
