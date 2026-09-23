import React, { useState } from 'react'

interface AvatarProps {
  src?: string
  initials?: string
  className?: string
  alt?: string
  style?: React.CSSProperties
}

export const Avatar: React.FC<AvatarProps> = ({ src, initials, className, alt = '', style }) => {
  const [imgError, setImgError] = useState(false)
  const showImg = src && !imgError

  return (
    <span
      data-slot="avatar"
      className={`inline-flex items-center justify-center overflow-hidden ${className ?? ''}`}
      style={style}
    >
      {showImg ? (
        <img
          src={src}
          alt={alt}
          className="h-full w-full object-cover"
          onError={() => setImgError(true)}
        />
      ) : (
        <span className="text-xs leading-none font-semibold text-white">{initials}</span>
      )}
    </span>
  )
}
