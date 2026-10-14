# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-10-13

### Added

- Attributes page: a grid of track crops for one label type, with number keys to assign attributes.
- Drift flags are stored by the API when the tracker writes boxes, and the Labels page shows the drift rate per type.
- Review queue: labelers flag frames with M, and developers and admins see open reviews and resolve them as accepted, fixed, or dismissed.
- Exemplar mode: Shift+X on a selected box finds the other objects like it on the frame with SAM 3 and tracks each as a new proposed track.
- SAM image embeddings are computed for each clip after ingest, and tracking skips the image encoder on embedded frames. `KRILL_EMBED_AT_INGEST=false` turns this off.
- Gold routing and history: labelers whose recent gold scores fall below `KRILL_GOLD_REVIEW_THRESHOLD` have their finished frames queued for review, and the Gold page lists past checks with the match view for each.
- Display-only CLAHE and white balance on the clip page. Exported images are never corrected.
- Videos can be held out as a test split, which exports write to their own folder.

### Changed

- Generated avatars replace initials.
- Shorter page and dialog subtitles.

### Fixed

- Toasts can be dismissed, repeats are collapsed, and they match the theme.
- Internal error details are no longer shown to users.
- Gold frame answers include boxes the tracker proposed.

## [0.1.0] - 2026-09-27

### Added

- Monorepo layout: proto, Go API, React web app, Python worker, and deploy config.
- Video upload in multipart chunks, with frame extraction, perceptual hashing, and clips of about 15 seconds.
- Clip viewer with frame stepping, playback, zoom, and a scrubbable timeline.
- Label types with track attributes, and manual box labeling with tracks, copy forward, and undo.
- YOLO export with per-video train/val split, frame stride, and perceptual-hash dedup.
- Accounts with password or Slack sign-in, and labeler, developer, and admin roles. Password sign-up needs a wisc.edu email.
- Leaderboards for today, this week, this month, and all time, and profiles with a contribution graph, streaks, and stats.
- Home page for labelers with a clip queue, soft clip claims, personal stats, and per-clip progress for every video.
- Click to track: the GPU worker tracks an object through the rest of a clip from a click or box with SAM 2.1, and saves its boxes as proposals.
- Tracked objects are left out of exports until the last frame of the track is confirmed.
- Drift flags on tracked boxes that jump in size or position, with D to jump to the next one.
- Label guides with example images, shown while tagging.
- Gold frames: developers and admins pick gold frames, labelers get a gold check every 100 frames, and each labeler's accuracy is scored.
- Sign-in rate limits can use a trusted proxy's client IP header, such as Cloudflare's `CF-Connecting-IP`.

[Unreleased]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Wisconsin-AUV/krill-monorepo/releases/tag/v0.1.0
