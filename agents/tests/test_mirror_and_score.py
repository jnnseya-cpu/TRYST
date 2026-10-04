import math

import pytest
from hypothesis import given
from hypothesis import strategies as st

from tryst_agents.broker.score import P1_WEIGHTS, p1_score, soft_min
from tryst_agents.mirror.vectors import ALPHA_MAX, ALPHA_MIN, alpha, decay_weight, effective_vector

FLOATS = st.floats(min_value=-50, max_value=50, allow_nan=False)


@given(st.lists(FLOATS, min_size=1, max_size=6).flatmap(
    lambda w: st.tuples(st.just(w), st.lists(FLOATS, min_size=len(w), max_size=len(w)))))
def test_alpha_respects_stated_preference_floor(wx):
    w, x = wx
    a = alpha(w, x)
    assert ALPHA_MIN <= a <= ALPHA_MAX


def test_personalisation_off_freezes_alpha():
    assert alpha([10.0], [10.0], personalisation_enabled=False) == ALPHA_MIN


def test_effective_vector_rejects_alpha_outside_bounds():
    with pytest.raises(ValueError):
        effective_vector([1.0], [0.0], 0.9)


def test_decay_half_life():
    assert decay_weight(0) == 1.0
    assert math.isclose(decay_weight(90), 0.5)


def test_p1_weights_sum_to_one():
    assert math.isclose(sum(P1_WEIGHTS.values()), 1.0)
    assert math.isclose(p1_score({k: 1.0 for k in P1_WEIGHTS}), 1.0)


@given(st.lists(st.floats(min_value=0, max_value=1), min_size=2, max_size=4))
def test_soft_min_is_bounded_by_edges(edges):
    s = soft_min(edges)
    assert min(edges) - 1e-9 <= s <= max(edges) + 1e-9


def test_one_strong_edge_cannot_mask_a_weak_one():
    assert soft_min([0.95, 0.1]) < 0.5
