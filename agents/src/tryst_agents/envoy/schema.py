"""Envoy negotiation and Brief schemas (02 §6.1-6.2). No free-text fields anywhere."""

from __future__ import annotations

import re
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field, field_validator

IntentShape = Literal["one_off", "recurring", "ongoing", "exploratory", "third", "couple"]
TierName = Literal["V0", "V1", "V2", "V3"]
_HHMM = re.compile(r"^([01]\d|2[0-3]):[0-5]\d$")


class Strict(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)


class Window(Strict):
    dow: list[int] = Field(min_length=1)
    start: str
    end: str
    tz: str

    @field_validator("dow")
    @classmethod
    def _dow(cls, v: list[int]) -> list[int]:
        if any(d < 1 or d > 7 for d in v):
            raise ValueError("dow must be 1..7")
        return v

    @field_validator("start", "end")
    @classmethod
    def _hhmm(cls, v: str) -> str:
        if not _HHMM.match(v):
            raise ValueError("expected HH:MM")
        return v


class Assertion(Strict):
    intent_shape: IntentShape
    structure: str
    notice_required_hours: int = Field(ge=0, le=24 * 30)
    availability: list[Window]  # member-provided windows only (FR-067)
    travel_radius_km: int = Field(ge=0, le=500)
    hard_limits: list[str] = []
    hosting_capability: bool = False
    verification_tier: TierName
    min_counterparty_tier: TierName = "V2"
    # Private, computable-only fields (never emitted): exclusion zones as geohash-6 cells.
    exclusion_zones: list[str] = []


class NegotiationTurn(Strict):
    turn: int = Field(ge=1, le=6)
    from_envoy: str
    to_envoy: str
    assertion: Assertion
    query: list[str] = []
    private_fields: list[str] = ["exclusion_zones"]


class Friction(Strict):
    field: str
    detail_code: str
    suggested_resolution: str


class Brief(Strict):
    label: Literal["agent_generated"] = "agent_generated"
    compatibility: float = Field(ge=0.0, le=1.0)
    aligned: list[str]
    friction: list[Friction]
    blocking: list[str]
    recommended_next: Literal["open_thread", "amend", "decline"]
    estimated_meet_probability: float = Field(ge=0.0, le=1.0)
