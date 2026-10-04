"""Envoy Briefs never leak private fields and stay schema-strict (FR-015, 02 §6)."""

import json

import pytest
from hypothesis import given
from hypothesis import strategies as st
from pydantic import ValidationError

from tryst_agents.envoy.negotiate import negotiate
from tryst_agents.envoy.schema import Assertion, NegotiationTurn

ZONE = st.from_regex(r"\Agc[0-9b-hjkmnp-z]{4}\Z", fullmatch=True)


def _assertion(**kw):
    base = {
        "intent_shape": "one_off",
        "structure": "attached_undisclosed",
        "notice_required_hours": 48,
        "availability": [{"dow": [2, 3], "start": "18:00", "end": "23:00", "tz": "Europe/London"}],
        "travel_radius_km": 25,
        "hard_limits": ["no_home_visit"],
        "verification_tier": "V2",
    }
    base.update(kw)
    return Assertion(**base)


@given(st.lists(ZONE, min_size=1, max_size=3), st.lists(ZONE, max_size=3))
def test_brief_never_contains_private_zones(my_zones, their_zones):
    brief = negotiate(_assertion(exclusion_zones=my_zones), _assertion(exclusion_zones=their_zones))
    dumped = json.dumps(brief.model_dump())
    for z in my_zones + their_zones:
        assert z not in dumped
    assert brief.label == "agent_generated"


def test_overlapping_zones_block_without_detail():
    brief = negotiate(_assertion(exclusion_zones=["gcpvj0"]), _assertion(exclusion_zones=["gcpvj0"]))
    assert brief.blocking == ["discretion_conflict"]
    assert brief.recommended_next == "decline"


def test_neither_hosting_is_friction_with_resolution():
    brief = negotiate(_assertion(), _assertion())
    assert any(f.detail_code == "neither_can_host" for f in brief.friction)


def test_no_free_text_fields_accepted():
    with pytest.raises(ValidationError):
        NegotiationTurn(turn=1, from_envoy="a", to_envoy="b", assertion=_assertion(),
                        message="hi, I'm really the user")  # extra field forbidden


def test_turn_limit_is_six():
    with pytest.raises(ValidationError):
        NegotiationTurn(turn=7, from_envoy="a", to_envoy="b", assertion=_assertion())
