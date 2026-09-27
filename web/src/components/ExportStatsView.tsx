import { ExclamationTriangleIcon } from '@heroicons/react/16/solid'
import { Badge } from '@/components/ui/Badge'
import type { ExportStats } from '@/gen/krill/v1/export_pb'
import { formatNumber, plural } from '@/lib/format'

function exportWarnings(stats: ExportStats): string[] {
  const warnings: string[] = []
  const images = stats.trainImages + stats.valImages
  if (images > 0 && stats.valImages === 0)
    warnings.push('No validation images. Pin a video to val or label another video.')
  if (stats.incompleteFrames > 0)
    warnings.push(
      `${plural(stats.incompleteFrames, 'frame')} left out because a track is missing an attribute.`,
    )
  if (stats.unverifiedFrames > 0)
    warnings.push(
      `${plural(stats.unverifiedFrames, 'frame')} left out because of unverified proposals.`,
    )
  const noVal = stats.classes.filter((c) => c.trainBoxes > 0 && c.valBoxes === 0).map((c) => c.name)
  if (stats.valImages > 0 && noVal.length > 0)
    warnings.push(`No val boxes for ${noVal.join(', ')}.`)
  return warnings
}

function Figure({ label, value, detail }: { label: string; value: number; detail?: string }) {
  return (
    <div className="rounded-lg bg-zinc-950/[2.5%] px-3 py-2.5 ring-1 ring-zinc-950/5 dark:bg-white/5 dark:ring-white/10">
      <div className="text-xs/5 text-zinc-500 dark:text-zinc-400">{label}</div>
      <div className="text-lg/7 font-semibold text-zinc-950 tabular-nums dark:text-white">
        {formatNumber(value)}
      </div>
      {detail && (
        <div className="text-xs/5 text-zinc-500 tabular-nums dark:text-zinc-400">{detail}</div>
      )}
    </div>
  )
}

export function ExportStatsView({ stats }: { stats: ExportStats }) {
  const used = stats.classes.filter((c) => c.trainBoxes + c.valBoxes > 0)
  const unused = stats.classes.length - used.length
  const warnings = exportWarnings(stats)
  const skipped = [
    ['not done', stats.unlabeledFrames],
    ['stride', stats.strideSkipped],
    ['duplicates', stats.duplicateSkipped],
    ['incomplete', stats.incompleteFrames],
    ['unverified', stats.unverifiedFrames],
  ].filter(([, n]) => Number(n) > 0)

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Figure
          label="Train images"
          value={stats.trainImages}
          detail={plural(stats.trainBoxes, 'box', 'boxes')}
        />
        <Figure
          label="Val images"
          value={stats.valImages}
          detail={plural(stats.valBoxes, 'box', 'boxes')}
        />
        <Figure label="Negatives" value={stats.negativeImages} detail="Images with no boxes" />
        <Figure label="Classes used" value={used.length} detail={`of ${stats.classes.length}`} />
      </div>

      {skipped.length > 0 && (
        <p className="text-sm/6 text-zinc-500 dark:text-zinc-400">
          Frames left out:{' '}
          {skipped.map(([label, n], i) => (
            <span key={String(label)}>
              {i > 0 && ' · '}
              <span className="text-zinc-700 tabular-nums dark:text-zinc-300">
                {formatNumber(Number(n))}
              </span>{' '}
              {label}
            </span>
          ))}
        </p>
      )}

      {warnings.length > 0 && (
        <ul className="space-y-1.5 rounded-lg bg-amber-50 p-3 ring-1 ring-amber-600/15 dark:bg-amber-400/10 dark:ring-amber-400/20">
          {warnings.map((w) => (
            <li key={w} className="flex gap-2 text-sm/6 text-amber-800 dark:text-amber-200">
              <ExclamationTriangleIcon className="mt-1 size-4 shrink-0 text-amber-500" />
              {w}
            </li>
          ))}
        </ul>
      )}

      {used.length > 0 && (
        <div>
          <h3 className="text-sm/6 font-medium text-zinc-950 dark:text-white">Classes</h3>
          <table className="mt-2 w-full text-left text-sm/6">
            <thead className="text-xs/6 text-zinc-500 dark:text-zinc-400">
              <tr className="border-b border-zinc-950/10 dark:border-white/10">
                <th className="py-1 font-medium">Class</th>
                <th className="py-1 text-right font-medium">Train</th>
                <th className="py-1 text-right font-medium">Val</th>
                <th className="py-1 text-right font-medium" title="Distinct clips with this class">
                  Clips
                </th>
                <th className="py-1 text-right font-medium" title="Distinct videos with this class">
                  Videos
                </th>
              </tr>
            </thead>
            <tbody className="text-zinc-700 tabular-nums dark:text-zinc-300">
              {used.map((c) => (
                <tr key={c.id} className="border-b border-zinc-950/5 dark:border-white/5">
                  <td className="py-1.5 font-mono text-[13px]">
                    <span className="mr-2 text-zinc-400">{c.id}</span>
                    {c.name}
                  </td>
                  <td className="py-1.5 text-right">{formatNumber(c.trainBoxes)}</td>
                  <td
                    className={
                      c.valBoxes === 0
                        ? 'py-1.5 text-right text-amber-600 dark:text-amber-400'
                        : 'py-1.5 text-right'
                    }
                  >
                    {formatNumber(c.valBoxes)}
                  </td>
                  <td className="py-1.5 text-right">{c.clips}</td>
                  <td className="py-1.5 text-right">{c.videos}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {unused > 0 && (
            <p className="mt-2 text-xs/5 text-zinc-500 dark:text-zinc-400">
              {unused} more {unused === 1 ? 'class has' : 'classes have'} no boxes and still get an
              id in data.yaml.
            </p>
          )}
        </div>
      )}

      {stats.videos.length > 0 && (
        <div>
          <h3 className="text-sm/6 font-medium text-zinc-950 dark:text-white">Videos</h3>
          <ul className="mt-2 divide-y divide-zinc-950/5 text-sm/6 dark:divide-white/5">
            {stats.videos.map((v) => (
              <li key={String(v.videoId)} className="flex items-center gap-3 py-1.5">
                <span className="min-w-0 flex-1 truncate text-zinc-700 dark:text-zinc-300">
                  {v.name}
                </span>
                <span className="text-xs text-zinc-500 tabular-nums dark:text-zinc-400">
                  {plural(v.images, 'image')} · {plural(v.boxes, 'box', 'boxes')}
                </span>
                <Badge color={v.split === 'val' ? 'violet' : 'sky'}>
                  {v.split}
                  {v.pinned && ' · pinned'}
                </Badge>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
