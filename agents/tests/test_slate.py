from hypothesis import given
from hypothesis import strategies as st

from tryst_agents.broker.slate import (
    SLATE_SIZE,
    Candidate,
    assemble_slate,
    clamp_cap,
)

cands = st.lists(
    st.builds(Candidate, id=st.text("abcdefghij", min_size=1, max_size=3),
              score=st.floats(0, 1), inbound_last_24h=st.integers(0, 50),
              inbound_cap=st.integers(0, 60)),
    max_size=40,
)


@given(cands, cands)
def test_slate_invariants(ranked, exploration):
    slate = assemble_slate(ranked, exploration)
    ids = [c.id for c, _ in slate]
    assert len(slate) <= SLATE_SIZE
    assert len(ids) == len(set(ids))
    assert all(c.inbound_last_24h < clamp_cap(c.inbound_cap) for c, _ in slate)


def test_exploration_floor_reserved():
    ranked = [Candidate(id=f"r{i}", score=1.0) for i in range(30)]
    exploration = [Candidate(id=f"e{i}", score=0.0) for i in range(10)]
    slate = assemble_slate(ranked, exploration)
    assert len(slate) == 18
    assert sum(1 for _, explore in slate if explore) >= 3  # ceil(18 * 0.15)


def test_cap_bounds():
    assert clamp_cap(1) == 4 and clamp_cap(100) == 40 and clamp_cap(12) == 12
