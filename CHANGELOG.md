# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Monorepo layout: proto, Go API, React web app, Python worker, and deploy config.
- Video upload with frame extraction, perceptual hashing, and 10 to 20 second clips.
- Clip viewer with frame stepping, playback, zoom, and a scrubbable timeline.
- Label types with track attributes, and manual box labeling with tracks, copy forward, and undo.
- YOLO export with per-video train/val split, frame stride, and perceptual-hash dedup.
- Accounts with password or Slack sign-in, and labeler, developer, and admin roles.
- Leaderboards for today, this week, this month, and all time, and profiles with a contribution graph, streaks, and stats.
- Home page for labelers with a clip queue, soft clip claims, personal stats, and per-clip progress for every video.
- Click to track: the GPU worker tracks an object through the rest of a clip from a click or box with SAM 2.1, and saves its boxes as proposals.
