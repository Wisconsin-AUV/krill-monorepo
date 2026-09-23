from krill_worker import __version__
from krill_worker.prelabelers import Proposal


def test_version() -> None:
    assert __version__


def test_proposal_defaults() -> None:
    p = Proposal(label="gate", box=(0.5, 0.5, 0.1, 0.1), confidence=0.9)
    assert p.angle is None
