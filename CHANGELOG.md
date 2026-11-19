# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Tracking keypoints of a type with a reference design takes about 45% less time per frame.
- Keypoints can be removed from or reordered in a type whose boxes have keypoints labeled. Boxes and gold frames keep each remaining keypoint where it was placed, and the type's settings say which keypoints saving clears and from how many boxes.
- A scheduled model is pinned on its own when the pinned model finds another kind of box, so the first pose or OBB model no longer has to be pinned by hand.

### Fixed

- Tracked and fit reference quads reach the corners of paper in the crate's shadow or curled toward its rim. Their corners are placed on SAM's mask of the whole sheet instead of only its brightest part.
- Fit keypoints finds sheets whose design runs to the paper's edge, which it skipped about half the time, and skips designs that look the same turned a quarter instead of guessing their corners' order.
- Tracked reference quads no longer drift off the sheet over a long track.

## [0.9.1] - 2026-11-17

### Changed

- Tracking from a frame with R uses the keypoints labeled on the track's earlier frames. Keypoints missing on the frame it starts from, or that tracking loses along the way, are put back where they sat on the printed sheet.
- A tracked sheet only jumps to a fresh find that matches its design clearly better, so a sheet half under the crate's wall no longer shrinks to its visible part.
- Dragging a keypoint moves its quad, center, and reference design with it instead of after the drop.

### Fixed

- Tracked keypoints on a crate's rim no longer slide down its near wall to where it meets the floor when the crate is seen at an angle.

## [0.9.0] - 2026-11-17

### Added

- Label types can have up to 64 keypoints, exported and trained as YOLO-pose. Each keypoint can name the one it swaps with when an image is mirrored, so flips keep left and right apart. Four keypoints can form a quad, the corners of a flat surface like a printed sheet, with an optional keypoint where its diagonals cross, and one quad can carry the type's reference design. A type can't be both oriented and have keypoints, and a type whose boxes have keypoints can only be merged into one with the same keypoints.
- Click to place the next keypoint, Alt+click to place it hidden, and drag keypoints to move them, with a magnifier while placing. O hides or shows a keypoint, X clears it, and W and Q step through them. The reference design is laid over its quad, so corners in the wrong order show up as a turned design.
- SAM tracking carries keypoints along with the box. It finds the reference design's quad on every frame by itself, and keeps keypoints a labeler placed on later frames instead of its own. Shift+click a surface inside the selected box to have SAM fill its quad. Keypoints that jump between frames get a drift flag like boxes do.
- Fit keypoints, in a type's settings, finds the reference design's quad in the boxes the type already had. The boxes it fills become proposals again for a labeler to check.
- YOLO-pose weights can be trained, uploaded, pinned, and trialed, and their pre-labels carry keypoints. Runs show pose mAP, and gold checks and model trials score keypoints by object keypoint similarity. A box with keypoints only counts as correct when they're close enough too.

### Changed

- When any type has keypoints, exports and training use YOLO-pose instead of YOLO-OBB. Boxes of oriented types are then written unrotated, and boxes of types without keypoints get unlabeled ones.
- A run can only be fine-tuned from, or compared against, a model with the same number of keypoints, so the first pose model has to be pinned by hand.
- Training and exports don't rotate images by default, since the box around a rotated object is looser than the object. Saturation varies by 0.4 instead of 0.7, so classes told apart by color stay apart.

### Fixed

- Objects pasted onto distractors keep their size, angle, and color. Training resized, rotated, and recolored them on top of what it does to every image, so small objects could shrink to a few pixels and lose their color.
- Light and shadow patches that overlap no longer black out or blow out what's under them.

## [0.8.0] - 2026-11-15

### Added

- Oriented label types, for objects like printed signs that sit at an angle. Their boxes are rotated rectangles, rotated with the handle above a selected box. When any type is oriented, exports and training use YOLO-OBB, and boxes of other types are written unrotated.
- SAM tracking fits a rotated box to the object on each frame for oriented types. An oriented type can have a reference design, the file its prints are made from, and boxes are then fit to the design's corners.
- Fit rotations, in an oriented type's settings, fits rotated boxes to the boxes it already had.
- YOLO-OBB weights can be uploaded, pinned, and trialed like detection weights, and their pre-labels and trial boxes are rotated.
- Augmentation settings when starting a training run, shown on the run's page. Besides Ultralytics' rotation, shift, zoom, shear, perspective, color, mosaic, and flips, Krill can stretch images, tint them blue-green like water, add patches of light and shadow, and add sensor noise. Val and test images are never changed.
- Exports can write augmented copies of each train image, with the same augmentation as training except mosaic, so models trained outside Krill get it too.
- A distractors augmentation that cuts the labeled objects out and pastes them at random spots on a random distractor image, each resized, rotated, stretched, and recolored on its own, while training and in exported copies. Admins upload distractor images under Pipeline in the admin panel, and the upload tray shows their progress. Each is turned upright and shrunk to 2048 pixels on its long side, so full-size camera photos don't fill storage. Ones that fail to process can be retried, and many can be selected and deleted at once.

### Changed

- Training no longer mirrors images by default, since a mirrored arrow or printed sign means something else, and it rotates images up to 10° either way. Krill's own stretch, tint, light and shadow, and noise are on by default, and so are distractors once any are uploaded.
- A run can only be fine-tuned from a model that finds the same kind of box, rotated or not. A scheduled run isn't compared against a pinned model of the other kind, so the first OBB model after making a type oriented has to be pinned by hand.

## [0.7.0] - 2026-11-13

### Added

- Upload YOLO weights trained outside Krill from the Models page. They become a run you can trial, pin, or fine-tune from. Their classes match label types by export class name, and classes Krill doesn't have are ignored. Only detection weights are accepted, not segmentation or pose.

### Changed

- The first account signs up as a labeler instead of an admin, so whoever reaches a new instance first can't take it over. Make an admin on the server with `krill user role <email or username> admin`. The API logs a warning at startup until an admin exists.
- The API image's binary is `krill` instead of `api`.

### Fixed

- The sign-in page marks the method you last used, whether that was your password or an emailed code, even when Slack sign-in is off.

## [0.6.1] - 2026-10-28

### Fixed

- Holding Cmd/Ctrl lets you draw a new box inside an existing one instead of dragging it.

## [0.6.0] - 2026-10-21

### Added

- Model trials: run any finished training run over a whole clip, held-out test videos included, and see its boxes in the clip view without pinning it or writing proposals. A slider thresholds the boxes live, compare mode colors them against the verified labels the way gold checks do, the timeline marks frames where they disagree, and the run page lists each trial's precision and recall per class.
- A trial can cover a whole video, one clip at a time, and the run page rolls the clips up into precision and recall per video.
- The run page lists the labeled frames a run's trials got most wrong, with how many boxes each missed, invented, or got loose or mislabeled, filtered by video or label type, and opens each frame in the clip view with the comparison on.
- Reviewers can start and cancel trials from the clip view, and labelers can open the trials already run on a clip.
- In a trial, B jumps to the next frame where the model disagrees with the labels, and Shift+B hides or shows its boxes.

### Changed

- The GPU worker keeps the pinned model loaded while it runs a trial with another one, so pre-labeling doesn't reload it afterwards. It holds up to two YOLO models in GPU memory.
- Trials have their own queue, so one doesn't wait behind an hours-long pre-labeling pass.

## [0.5.0] - 2026-10-15

### Added

- Admin panel at `/admin` with an overview, users, settings, and compute.
- The overview shows whether a newer release is out, with its changelog, and checks the database, storage, GPU worker, email, and Slack. `KRILL_UPDATE_CHECK=false` stops it asking GitHub.
- Settings can be changed in the admin panel and apply without a restart. Variables set in the environment still win.
- The compute page shows how full MinIO's drive is, charts CPU, memory, disk, and GPU use for the API and each GPU worker, and shows what the worker is running. The overview warns once MinIO is 90% full.
- Admins can search users, sign a user out everywhere, and mark an email verified.
- A test email can be sent from the settings page.
- A help menu in the top bar lists what's new and opens the changelog in the app, with a dot until you've seen the latest release.
- An audit log in the admin panel records who changed users, settings, videos, clips, label types, exports, gold frames, and reviews, and when. Secret settings are logged as changed without their value.
- The settings page checks the Slack client ID and secret with Slack, and runs a test Slack sign-in that comes back with the workspace without signing you in. Problems name the setting to fix, or the exact redirect URL to add in Slack.
- An emails page in the admin panel previews every email as it is sent, in HTML and text, and sends any of them to you.
- A labelers page shows each person's frames, drawn boxes, proposals kept and rejected, how often reviewers corrected their frames, active time, and gold accuracy for a day, week, month, or all time, with trends per person.
- A coverage page counts verified and proposed boxes per label type and video, leaves held-out test videos out of the totals, and flags label types with few boxes or few videos.
- A stuck clips page lists claimed clips nobody has edited for a few days, `KRILL_STUCK_CLAIM_DAYS` by default, and releases them back to the queue.
- Invite links with a role, a number of uses, and an expiry, which work while sign-up is closed and with Sign in with Slack. Admins can email the link, see who used it, and revoke it.
- Pre-approved emails can sign up once with a set role, whatever the sign-up settings.
- Admins can deactivate an account, which signs it out everywhere, blocks every way of signing in, and frees its clips. Its labels keep its name.
- Admins can email a user a password reset code.
- Admins can merge two accounts that belong to the same person. A preview shows what moves, and the kept account gets the other's sign-in methods and the higher role.
- Each user has a page in the admin panel with their sessions and history.
- A sessions page shows who is signed in, from which browser and address, and when they were last active, and revokes any session. Everyone can see and revoke their own sessions on the Account page.
- A jobs page lists every ingest, embedding, tracking, similar-object search, and export with its state, attempts, errors, timing, who started it, and which GPU worker runs it. Admins can retry, cancel, and reprioritize jobs. Cancelling a GPU job stops it on the worker.
- The SAM model is a setting in the admin panel, from SAM 2.1 tiny to large or SAM 3. Workers switch between tasks unless their own `KRILL_SAM_MODEL` pins one.
- The compute page shows each worker's model, whether it's loaded, the GPU memory it holds, the models it has downloaded, its token, and its last heartbeat, with buttons to warm the model up or unload it.
- Worker tokens can be created and revoked in the admin panel, alongside `KRILL_WORKER_TOKEN`, so a worker's token can be rotated without restarting the API.
- A storage page breaks the bucket down by kind and by video, finds objects no row refers to and rows whose object is gone, and deletes the orphans after checking each one again.
- A database page shows table sizes, row counts, and migration versions, and backs the database up to MinIO with `pg_dump`, by hand or daily with `KRILL_BACKUP_SCHEDULE`. Scheduled backups beyond `KRILL_BACKUP_KEEP` are deleted, and the overview warns when the last one failed or is over a day old.
- A logs page shows recent log records from the API and each GPU worker, with level and text filters and a live tail. Emails logged in development show their code there. Tokens, passwords, and keys are redacted.
- Deleted videos and clips go to a trash in the admin panel, where they can be restored with their labels or deleted for good. Deleting shows an Undo button, and the trash empties itself after `KRILL_TRASH_RETENTION_DAYS` (30 by default, 0 keeps everything).
- A label type can be merged into another, mapping its attribute values onto the other type's. The preview shows the tracks, boxes, gold frames, and export classes it changes.
- A Content page in the admin panel re-extracts frames, regenerates SAM embeddings for stale or chosen clips, and assigns or releases clips in bulk, with the progress of each job.
- Admins can train YOLO on verified labels from the Models page, follow each epoch, and compare per-class precision, recall, and mAP on the validation and held-out test videos against the pinned model. Runs can be cancelled, and the GPU worker pauses training between batches whenever someone tracks, so clicks don't wait.
- Retraining can run daily at a set hour once enough new labels are verified, and a scheduled model that beats the pinned one on the held-out test videos is pinned on its own.
- Pinning a model, or rolling back to an older one, pre-labels every unlabeled frame outside the held-out test videos. Its boxes show as YOLO proposals with their confidence and go through the same accept and reject flow. A rejected proposal isn't proposed again on that frame, nor is one moved far from where it was. Proposals whose label or attributes a labeler changed stay through later runs. Moving a video to the test split removes its YOLO and CV proposals.
- Classic CV pre-labelers for orange path markers and red and white slalom pipes, with HSV thresholds tuned on the Pre-labelers page against any frame.
- Export defaults for the validation share, frame stride, and duplicate threshold are settings, and prefill the export form.

### Changed

- Users moved from the top bar into the admin panel, and `/users` redirects there.
- Disabling a user is now called deactivating, and also frees the clips they were working on.
- The version moved from the top bar into the help menu.
- An empty `KRILL_` variable now counts as unset. To accept any email domain from the environment, set `KRILL_EMAIL_VERIFICATION=off`.
- Docker Compose no longer fills in defaults for settings the admin panel can change.
- The compute page's queue counts GPU jobs waiting to start, not only the one waiting for a worker.
- A clip whose embedding is interrupted by a model switch is embedded again with the new model instead of failing.
- The API image includes `pg_dump` 17 for backups.
- Trashed videos and clips are left out of the queue, exports, stats, gold, and reviews until they are restored.
- The clip queue starts with the clip whose pre-labels are least certain when videos are otherwise tied.
- Pre-label tracks no longer wait for their last box to be accepted before exporting, since they can't drift like tracked boxes.

### Fixed

- Icons in menus turn white with the highlighted item instead of staying gray, and follow the system colors in high contrast mode.

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

[Unreleased]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.9.1...HEAD
[0.9.1]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.6.1...v0.7.0
[0.6.1]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/Wisconsin-AUV/krill-monorepo/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Wisconsin-AUV/krill-monorepo/releases/tag/v0.1.0
