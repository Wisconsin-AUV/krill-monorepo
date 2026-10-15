export interface ReleaseSection {
  title: string
  items: string[]
}

export interface Release {
  // vX.Y.Z, or "Unreleased" for changes after the latest tag.
  version: string
  date: string
  sections: ReleaseSection[]
}

const releaseHeading = /^## \[([^\]]+)\](?:\s*-\s*(\S+))?/
const sectionHeading = /^### (.+)/

// Reads a Keep a Changelog file, newest release first, skipping empty ones.
export function parseChangelog(md: string): Release[] {
  const out: Release[] = []
  let release: Release | null = null
  let section: ReleaseSection | null = null
  for (const line of md.split('\n')) {
    const r = releaseHeading.exec(line)
    if (r) {
      const name = r[1]
      release = {
        version: /^\d/.test(name) ? `v${name}` : name,
        date: r[2] ?? '',
        sections: [],
      }
      section = null
      out.push(release)
      continue
    }
    if (!release) continue
    const s = sectionHeading.exec(line)
    if (s) {
      section = { title: s[1].trim(), items: [] }
      release.sections.push(section)
    } else if (section && line.startsWith('- ')) {
      section.items.push(line.slice(2).trim())
    } else if (section && /^\s{2,}\S/.test(line) && section.items.length > 0) {
      section.items[section.items.length - 1] += ` ${line.trim()}`
    }
  }
  return out
    .map((r) => ({ ...r, sections: r.sections.filter((s) => s.items.length > 0) }))
    .filter((r) => r.sections.length > 0)
}

// The release a build is from, from a version like v0.4.0-6-g2d343e3.
export function releaseOf(version: string): string | null {
  const m = /^v?(\d+\.\d+\.\d+)/.exec(version)
  return m ? `v${m[1]}` : null
}

const dateFormat = new Intl.DateTimeFormat(undefined, {
  month: 'short',
  day: 'numeric',
  year: 'numeric',
})

export function formatReleaseDate(date: string): string {
  const d = new Date(`${date}T00:00:00`)
  return Number.isNaN(d.getTime()) ? date : dateFormat.format(d)
}

// A one-line summary of a release for menus: its first addition, or else
// its first change of any kind, without markdown.
export function headline(r: Release): string {
  const s = r.sections.find((x) => x.title === 'Added') ?? r.sections[0]
  return (s?.items[0] ?? '').replace(/`([^`]+)`/g, '$1').replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
}
