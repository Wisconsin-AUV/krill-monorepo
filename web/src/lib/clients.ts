import { createClient } from '@connectrpc/connect'
import { AnnotationService } from '@/gen/krill/v1/annotation_pb'
import { LabelService } from '@/gen/krill/v1/label_pb'
import { VideoService } from '@/gen/krill/v1/video_pb'
import { transport } from './transport'

export const videoClient = createClient(VideoService, transport)
export const labelClient = createClient(LabelService, transport)
export const annotationClient = createClient(AnnotationService, transport)
