"""Property tests: Stage 0 hard gates are inviolable (FR-014, 03 §5.5)."""

from hypothesis import given, settings
from hypothesis import strategies as st

from tryst_agents.broker.entities import Entity, Tier
from tryst_agents.broker.gates import (
    ALLOWED_JURISDICTIONS,
    eligible_candidates,
    mutual_eligible,
    visible_to,
)

IDS = ["a", "b", "c", "d"]
SHAPES = ["one_off", "recurring", "ongoing", "exploratory", "third", "couple"]
TAGS = ["t1", "t2", "t3"]
CELLS = ["gcpvj", "gcpuv", "u10hb"]
ACCOUNTS = ["acc1", "acc2", "acc3", "acc4"]


@st.composite
def entities(draw, eid=None):
    etype = draw(st.sampled_from(["solo", "couple"]))
    segment = draw(st.sampled_from(["S1", "S2", "S4", "S6"] if etype == "solo" else ["S3", "S5"]))
    zone_parent = draw(st.sampled_from(CELLS))
    return Entity(
        id=eid or draw(st.sampled_from(IDS)),
        entity_type=etype,
        segment=segment,
        tier=draw(st.sampled_from(list(Tier))),
        min_counterparty_tier=draw(st.sampled_from([Tier.V2, Tier.V3])),
        modes=frozenset(draw(st.sets(st.sampled_from(SHAPES), max_size=3))),
        hard_limits=frozenset(draw(st.sets(st.sampled_from(["no_couples", "no_solo_third", "no_overnight"]), max_size=2))),
        desire_tags=draw(st.dictionaries(st.sampled_from(TAGS), st.sampled_from(["yes", "curious", "no"]), max_size=3)),
        location_cell=draw(st.sampled_from(CELLS)),
        zones=frozenset(draw(st.sets(st.just(zone_parent + "x"), max_size=1))),
        jurisdiction=draw(st.sampled_from(["GB", "IE", "AE"])),
        visibility=draw(st.sampled_from(["public", "incognito"])),
        couple_valid=draw(st.booleans()) if etype == "couple" else True,
        safety_state=draw(st.sampled_from(["normal", "normal", "frozen"])),
        blocked=frozenset(draw(st.sets(st.sampled_from(IDS), max_size=2))),
        excluded=frozenset(draw(st.sets(st.sampled_from(IDS), max_size=2))),
        outbound_intents=frozenset(draw(st.sets(st.sampled_from(IDS), max_size=2))),
        owners=frozenset(draw(st.sets(st.sampled_from(ACCOUNTS), min_size=1, max_size=1 if etype == "solo" else 2))),
        shielded_accounts=frozenset(draw(st.sets(st.sampled_from(ACCOUNTS), max_size=2))),
    )


def _violations(a: Entity, b: Entity) -> list[str]:
    """Independent re-statement of every hard rule; any hit must make the pair ineligible."""
    v = []
    if a.tier < Tier.V2 or b.tier < Tier.V2:
        v.append("below V2 (FR-001)")
    if a.tier < b.min_counterparty_tier or b.tier < a.min_counterparty_tier:
        v.append("tier parity")
    if ("no_couples" in a.hard_limits and b.entity_type == "couple") or (
        "no_couples" in b.hard_limits and a.entity_type == "couple"):
        v.append("hard limit no_couples")
    for t in set(a.desire_tags) & set(b.desire_tags):
        if {a.desire_tags[t], b.desire_tags[t]} == {"yes", "no"}:
            v.append("desire-tag collision")
    if a.id in b.excluded or b.id in a.excluded:
        v.append("exclusion ring")
    if a.id in b.blocked or b.id in a.blocked:
        v.append("block")
    if a.owners & b.owners:
        v.append("same person on both sides")
    if a.owners & b.shielded_accounts or b.owners & a.shielded_accounts:
        v.append("partner secret")
    if any(z.startswith(b.location_cell) for z in a.zones) or any(z.startswith(a.location_cell) for z in b.zones):
        v.append("geofence")
    if a.safety_state != "normal" or b.safety_state != "normal":
        v.append("safety state")
    if a.jurisdiction not in ALLOWED_JURISDICTIONS or b.jurisdiction not in ALLOWED_JURISDICTIONS:
        v.append("jurisdiction")
    for e in (a, b):
        if e.entity_type == "couple" and not e.couple_valid:
            v.append("couple not co-signed (G-NG-5)")
    if not (a.modes & b.modes):
        v.append("no shared mode")
    return v


@settings(max_examples=3000)
@given(entities(eid="a"), entities(eid="b"))
def test_no_eligible_pair_violates_a_hard_rule(a, b):
    ok, _ = mutual_eligible(a, b)
    if ok:
        assert _violations(a, b) == []


@settings(max_examples=2000)
@given(entities(eid="a"), entities(eid="b"))
def test_mutual_gates_are_symmetric(a, b):
    assert mutual_eligible(a, b)[0] == mutual_eligible(b, a)[0]


@given(entities(eid="a"), entities(eid="b"))
def test_incognito_only_visible_after_their_intent(viewer, cand):
    if cand.visibility == "incognito" and viewer.id not in cand.outbound_intents:
        assert not visible_to(viewer, cand)
        assert cand not in eligible_candidates(viewer, [cand])


def test_known_good_pair_is_eligible():
    base = {"entity_type": "solo", "tier": Tier.V2, "modes": frozenset({"one_off"}), "location_cell": "gcpvj"}
    a = Entity(id="a", segment="S1", **base)
    b = Entity(id="b", segment="S1", **base)
    assert mutual_eligible(a, b) == (True, "")


def test_third_requires_couple_and_s4():
    couple = Entity(id="c", entity_type="couple", segment="S3", tier=Tier.V2, modes=frozenset({"third"}))
    solo = Entity(id="s", entity_type="solo", segment="S4", tier=Tier.V2, modes=frozenset({"third"}))
    s1 = Entity(id="x", entity_type="solo", segment="S1", tier=Tier.V2, modes=frozenset({"third"}))
    assert mutual_eligible(couple, solo)[0]
    assert mutual_eligible(couple, s1) == (False, "structure")


def test_unsigned_couple_never_eligible():
    couple = Entity(id="c", entity_type="couple", segment="S3", tier=Tier.V2,
                    modes=frozenset({"third"}), couple_valid=False)
    solo = Entity(id="s", entity_type="solo", segment="S4", tier=Tier.V2, modes=frozenset({"third"}))
    assert mutual_eligible(couple, solo) == (False, "entity_valid")


def test_secret_affair_stays_secret():
    """Alice and Bob have a couple profile together. Alice also has a private solo profile.
    Bob never sees it, Alice's solo profile never sees the couple or Bob's own solo
    profile, and a stranger can still match Alice's solo profile."""
    base = {"tier": Tier.V2, "location_cell": "gcpvj", "modes": frozenset({"one_off", "third", "exploratory"})}
    alice_solo = Entity(id="alice-solo", entity_type="solo", segment="S1", owners=frozenset({"alice"}),
                        shielded_accounts=frozenset({"bob"}), **base)
    bob_solo = Entity(id="bob-solo", entity_type="solo", segment="S1", owners=frozenset({"bob"}),
                      shielded_accounts=frozenset({"alice"}), **base)
    couple = Entity(id="ab", entity_type="couple", segment="S3", owners=frozenset({"alice", "bob"}), **base)
    stranger = Entity(id="s", entity_type="solo", segment="S4", owners=frozenset({"carol"}), **base)
    assert mutual_eligible(alice_solo, bob_solo) == (False, "partner_secret")
    assert mutual_eligible(alice_solo, couple) == (False, "partner_secret")
    assert mutual_eligible(couple, alice_solo) == (False, "partner_secret")
    assert mutual_eligible(alice_solo, stranger) == (True, "")
    assert mutual_eligible(couple, stranger) == (True, "")


def test_partner_shield_without_partner_on_tryst_yet():
    """Dan added his wife's number to his partner shield. If she ever joins, PSI marks her
    account and neither ever sees the other, whatever their settings."""
    base = {"entity_type": "solo", "segment": "S1", "tier": Tier.V2, "modes": frozenset({"one_off"}),
            "location_cell": "gcpvj"}
    dan = Entity(id="dan", owners=frozenset({"dan"}), shielded_accounts=frozenset({"erin"}), **base)
    erin = Entity(id="erin", owners=frozenset({"erin"}), **base)
    assert mutual_eligible(dan, erin) == (False, "partner_secret")
    assert mutual_eligible(erin, dan) == (False, "partner_secret")
