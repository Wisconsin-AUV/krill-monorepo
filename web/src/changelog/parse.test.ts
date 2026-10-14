import { describe, expect, it } from 'vitest'
import { headline, parseChangelog, releaseOf } from './parse'

const md = `# Changelog

## [Unreleased]

## [0.5.0] - 2026-11-01

### Added

- An \`/admin\` panel that wraps
  onto a second line.

### Fixed

- A [bug](https://example.com).

## [0.4.0] - 2026-10-14

### Changed

- Something.
`

describe('parseChangelog', () => {
  it('reads releases newest first, joins wrapped items, and drops empty ones', () => {
    const releases = parseChangelog(md)
    expect(releases.map((r) => r.version)).toEqual(['v0.5.0', 'v0.4.0'])
    expect(releases[0].date).toBe('2026-11-01')
    expect(releases[0].sections.map((s) => s.title)).toEqual(['Added', 'Fixed'])
    expect(releases[0].sections[0].items).toEqual([
      'An `/admin` panel that wraps onto a second line.',
    ])
  })

  it('summarizes a release without markdown', () => {
    const [latest, older] = parseChangelog(md)
    expect(headline(latest)).toBe('An /admin panel that wraps onto a second line.')
    expect(headline(older)).toBe('Something.')
  })
})

describe('releaseOf', () => {
  it('finds the release a build came from', () => {
    expect(releaseOf('v0.4.0-6-g2d343e3-dirty')).toBe('v0.4.0')
    expect(releaseOf('0.4.0')).toBe('v0.4.0')
    expect(releaseOf('dev-2d343e3')).toBeNull()
  })
})
