import { useQuery } from '@connectrpc/connect-query'
import { useEffect, useRef } from 'react'
import { PipelineService } from '@/gen/krill/v1/pipeline_pb'
import { trialActive } from '@/admin/pipeline'
import { flash } from '@/lib/flash'
import { useTrialStore } from './useTrialStore'

const POLL_MS = 1500

// Boxes stream in while the worker runs, so the trial is polled until it
// finishes and the store always holds everything found so far.
export function useTrialPoll(trialId: bigint | undefined, clipId: bigint | undefined) {
  const { data, error } = useQuery(
    PipelineService.method.getModelTrial,
    { id: trialId },
    {
      enabled: trialId !== undefined,
      refetchInterval: (q) =>
        q.state.data?.trial && trialActive(q.state.data.trial) ? POLL_MS : false,
    },
  )

  useEffect(() => {
    const { load, clear } = useTrialStore.getState()
    if (trialId !== undefined && data?.trial?.id === trialId && data.trial.clipId === clipId)
      load(data.trial, data.boxes)
    else clear()
  }, [data, trialId, clipId])

  // A failing poll fails again every interval, so each error is shown once.
  const shown = useRef<string | undefined>(undefined)
  useEffect(() => {
    if (!error) shown.current = undefined
    else if (shown.current !== error.message) {
      shown.current = error.message
      flash.error('Could not load model trial', error)
    }
  }, [error])

  useEffect(() => () => useTrialStore.getState().clear(), [])
}
