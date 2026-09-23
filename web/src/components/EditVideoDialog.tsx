import { useMutation } from '@connectrpc/connect-query'
import { useState } from 'react'
import { Button } from '@/components/ui/Button'
import { Dialog, DialogActions, DialogBody, DialogTitle } from '@/components/ui/Dialog'
import { Description, Field, FieldGroup, Label } from '@/components/ui/Fieldset'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Textarea } from '@/components/ui/Textarea'
import { VideoService, type SplitAssignment, type Video } from '@/gen/krill/v1/video_pb'
import { flash } from '@/lib/flash'
import { invalidateService } from '@/lib/queryClient'
import { splitOptions } from '@/lib/video'

function EditVideoForm({ video, onClose }: { video: Video; onClose: () => void }) {
  const [name, setName] = useState(video.name)
  const [notes, setNotes] = useState(video.notes)
  const [split, setSplit] = useState(video.split)
  const update = useMutation(VideoService.method.updateVideo, {
    onSuccess: async () => {
      await invalidateService(VideoService)
      onClose()
    },
    onError: (err) => flash.error('Could not save video', err),
  })

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        update.mutate({ id: video.id, name, notes, split })
      }}
    >
      <DialogTitle>Edit video</DialogTitle>
      <DialogBody>
        <FieldGroup>
          <Field>
            <Label>Name</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              maxLength={200}
            />
          </Field>
          <Field>
            <Label>Notes</Label>
            <Description>Where and how it was shot: pool, depth, lighting, turbidity.</Description>
            <Textarea value={notes} onChange={(e) => setNotes(e.target.value)} rows={3} />
          </Field>
          <Field>
            <Label>Dataset split</Label>
            <Description>{splitOptions.find((o) => o.value === split)?.description}</Description>
            <Select
              value={split}
              onChange={(e) => setSplit(Number(e.target.value) as SplitAssignment)}
            >
              {splitOptions.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </Select>
          </Field>
        </FieldGroup>
      </DialogBody>
      <DialogActions>
        <Button plain onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" color="sky" disabled={update.isPending || !name.trim()}>
          Save
        </Button>
      </DialogActions>
    </form>
  )
}

export function EditVideoDialog({
  video,
  open,
  onClose,
}: {
  video: Video
  open: boolean
  onClose: () => void
}) {
  return (
    <Dialog open={open} onClose={onClose}>
      {open && <EditVideoForm video={video} onClose={onClose} />}
    </Dialog>
  )
}
