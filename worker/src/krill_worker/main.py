import logging

from krill_worker import __version__

log = logging.getLogger(__name__)


def main() -> None:
    logging.basicConfig(level=logging.INFO)
    log.info("krill-worker %s starting", __version__)


if __name__ == "__main__":
    main()
