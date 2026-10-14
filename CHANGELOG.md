# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Admin panel at `/admin` with an overview, users, settings, and compute.
- The overview shows whether a newer release is out, with its changelog, and checks the database, storage, GPU worker, email, and Slack. `KRILL_UPDATE_CHECK=false` stops it asking GitHub.
- Settings can be changed in the admin panel and apply without a restart. Variables set in the environment still win.
- The compute page shows how full MinIO's drive is, charts CPU, memory, disk, and GPU use for the API and each GPU worker, and shows what the worker is running. The overview warns once MinIO is 90% full.
- Admins can search users, sign a user out everywhere, and mark an email verified.
- A test email can be sent from the settings page.
- A help menu in the top bar lists what's new and opens the changelog in the app, with a dot until you've seen the latest release.
- An audit log in the admin panel records who changed users, settings, videos, clips, label types, exports, gold frames, and reviews, and when. Secret settings are logged as changed without their value.

### Changed

- Users moved from the top bar into the admin panel, and `/users` redirects there.
- The version moved from the top bar into the help menu.
- An empty `KRILL_` variable now counts as unset. To accept any email domain from the environment, set `KRILL_EMAIL_VERIFICATION=off`.
- Docker Compose no longer fills in defaults for settings the admin panel can change.

## [0.4.0] - 2026-10-14

### Added

- Developers and admins can delete a clip from the labeling view. The clips after it are renumbered and the next one opens.
- Opening a clip someone else is labeling asks before taking it over.

### Changed

- Gold is a tab on the Review page, and `/gold` redirects there.
- SAM tracking is about 40% faster on GPUs without fused attention kernels, such as the RX 6600.
- The timeline no longer redraws on every playhead or hover move.

### Fixed

- Holding space to mark frames done keeps up instead of trailing the server, and frames that fail to save are reverted.
- Every label edit keeps the clip claimed, so a clip is no longer handed out while someone is still working on it.
- Reviewer fixes no longer take a clip from the labeler working on it.

## [0.3.0] - 2026-10-14

### Added

- Email verification: with `KRILL_EMAIL_VERIFICATION=verify`, new accounts enter a code emailed to them. Emails are sent through Resend with `KRILL_RESEND_API_KEY` and `KRILL_EMAIL_FROM`.
- Password reset with an emailed code.
- Sign in with an emailed code.
- Changing your email takes effect once a code sent to the new address is entered.

### Changed

- Tracked boxes are dropped once only a sliver of the object is left at the frame edge.

### Removed

- Guideline link in the top navigation bar.

### Fixed

- Buttons keep their height while showing a loading spinner.
- Deleting a single box is visible and can be done from the track panel.

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

[Unreleased]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Wisconsin-AUV/krill-monorepo/releases/tag/v0.1.0
