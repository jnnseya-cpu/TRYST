"""Deterministic six-dimension scoring that produces asymmetric Briefs (03 §3.1).

Envoy never messages a human and cannot open a thread (FR-015, FR-055). The Brief is
built only from codes and field names, never from the counterparty's private fields.
"""

from __future__ import annotations

from .schema import Assertion, Brief, Friction

_TIER_ORDER = {"V0": 0, "V1": 1, "V2": 2, "V3": 3}
_COMPATIBLE_SHAPES = {
    frozenset({"one_off"}),
    frozenset({"recurring"}),
    frozenset({"ongoing"}),
    frozenset({"recurring", "ongoing"}),
    frozenset({"third"}),
    frozenset({"couple"}),
}


def _minutes(hhmm: str) -> int:
    h, m = hhmm.split(":")
    return int(h) * 60 + int(m)


def _windows_overlap(a: Assertion, b: Assertion) -> bool:
    for wa in a.availability:
        for wb in b.availability:
            if wa.tz != wb.tz or not set(wa.dow) & set(wb.dow):
                continue
            if _minutes(wa.start) < _minutes(wb.end) and _minutes(wb.start) < _minutes(wa.end):
                return True
    return False


def _zones_disjoint(a: Assertion, b: Assertion) -> bool:
    za, zb = set(a.exclusion_zones), set(b.exclusion_zones)
    return not any(x.startswith(y) or y.startswith(x) for x in za for y in zb)


def negotiate(me: Assertion, them: Assertion) -> Brief:
    """Brief for `me` about a handshake with `them`."""
    aligned: list[str] = []
    friction: list[Friction] = []
    blocking: list[str] = []

    shapes = frozenset({me.intent_shape, them.intent_shape})
    if "exploratory" in shapes or shapes in _COMPATIBLE_SHAPES:
        aligned.append("intent_shape")
    else:
        friction.append(Friction(field="intent_shape", detail_code="intent_mismatch",
                                 suggested_resolution="clarify_intent"))

    # Hard-limit and desire-tag collisions are excluded upstream by Broker's Stage 0 gates
    # (broker/gates.py), so any pair reaching a handshake has intersecting boundaries.
    aligned.append("boundaries")

    if _windows_overlap(me, them):
        aligned.append("availability")
    else:
        friction.append(Friction(field="availability", detail_code="no_window_overlap",
                                 suggested_resolution="propose_new_window"))

    if min(me.travel_radius_km, them.travel_radius_km) > 0:
        aligned.append("travel_radius")
    if not (me.hosting_capability or them.hosting_capability):
        friction.append(Friction(field="hosting", detail_code="neither_can_host",
                                 suggested_resolution="hotel_or_venue"))

    if _zones_disjoint(me, them):
        aligned.append("discretion")
    else:
        blocking.append("discretion_conflict")  # no detail: zones are private

    parity = (_TIER_ORDER[me.verification_tier] >= _TIER_ORDER[them.min_counterparty_tier]
              and _TIER_ORDER[them.verification_tier] >= _TIER_ORDER[me.min_counterparty_tier])
    if parity:
        aligned.append("verification")
    else:
        blocking.append("verification_parity")

    total = 6
    compatibility = round(len(aligned) / total, 2) if not blocking else 0.0
    if blocking:
        nxt = "decline"
    elif friction:
        nxt = "amend" if len(friction) > 1 else "open_thread"
    else:
        nxt = "open_thread"
    meet_p = round(compatibility * (0.5 if friction else 0.6), 2)
    return Brief(compatibility=compatibility, aligned=aligned, friction=friction,
                 blocking=blocking, recommended_next=nxt, estimated_meet_probability=meet_p)
