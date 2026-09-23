import { createClient } from '@connectrpc/connect'
import { VideoService } from '@/gen/krill/v1/video_pb'
import { transport } from './transport'

export const videoClient = createClient(VideoService, transport)
