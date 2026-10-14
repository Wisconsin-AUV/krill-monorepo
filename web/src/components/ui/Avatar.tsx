import BoringAvatar from 'boring-avatars'
import React from 'react'

const COLORS = ['#0c4a6e', '#0284c7', '#38bdf8', '#bae6fd', '#f59e0b']

interface AvatarProps {
  seed: string
  className?: string
  style?: React.CSSProperties
}

export const Avatar: React.FC<AvatarProps> = ({ seed, className, style }) => (
  <span
    data-slot="avatar"
    className={`inline-flex overflow-hidden rounded-full ${className ?? ''}`}
    style={style}
  >
    <BoringAvatar name={seed} variant="beam" colors={COLORS} size="100%" />
  </span>
)
