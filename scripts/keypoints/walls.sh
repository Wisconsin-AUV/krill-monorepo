#!/usr/bin/env bash
# Score how keypoints treat sheet corners a crate's wall hides, on the tracks
# labeled in walls.json. See walls.py.
#
#   scripts/keypoints/walls.sh
#   scripts/keypoints/walls.sh --verbose --model facebook/sam2.1-hiera-base-plus
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
deploy=${KRILL_DEPLOY:-/srv/krill/app/deploy}
project=${KRILL_PROJECT:-krill-server}
label_type=$(sed -n 's/.*"label_type": "\(.*\)".*/\1/p' "$here/walls.json")
image=${KRILL_WORKER_IMAGE:-$(docker inspect --format '{{.Config.Image}}' "$project-worker-1")}
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT

# deploy/.env has unquoted values with spaces, so it can't be sourced.
env_value() { grep -m1 "^$1=" "$deploy/.env" | cut -d= -f2-; }
export KRILL_S3_ACCESS_KEY KRILL_S3_SECRET_KEY KRILL_S3_BUCKET
KRILL_S3_ACCESS_KEY=$(env_value KRILL_S3_ACCESS_KEY)
KRILL_S3_SECRET_KEY=$(env_value KRILL_S3_SECRET_KEY)
KRILL_S3_BUCKET=$(env_value KRILL_S3_BUCKET || true)

docker exec -i "$project-postgres-1" sh -c \
    'psql -U "$POSTGRES_USER" -d "${POSTGRES_DB:-$POSTGRES_USER}" -At -v ON_ERROR_STOP=1 -v type="$0"' \
    "$label_type" <"$here/export.sql" >"$out/labels.json"

docker run --rm --network "${project}_default" \
    --device /dev/kfd --device /dev/dri -e HSA_OVERRIDE_GFX_VERSION=10.3.0 \
    -e PYTHONPATH=/src -e HF_HUB_OFFLINE=1 \
    -e KRILL_S3_ACCESS_KEY -e KRILL_S3_SECRET_KEY -e KRILL_S3_BUCKET \
    -v "$repo/worker/src:/src:ro" -v "$here:/kp:ro" -v "$out:/out:ro" \
    -v krill-hf:/root/.cache/huggingface \
    --entrypoint python "$image" \
    /kp/walls.py --labels /out/labels.json --tracks /kp/walls.json "$@"
