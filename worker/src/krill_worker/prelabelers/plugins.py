from krill_worker.prelabelers.cv import CvPreLabeler
from krill_worker.prelabelers.path_marker import PathMarker
from krill_worker.prelabelers.slalom import Slalom

# Every classic CV pre-labeler the worker offers, by name.
PLUGINS: dict[str, type[CvPreLabeler[object]]] = {
    p.name: p  # type: ignore[misc]
    for p in (PathMarker, Slalom)
}
