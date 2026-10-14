import type { ReactNode } from 'react'
import {
  Body,
  Container,
  Head,
  Heading,
  Hr,
  Html,
  pixelBasedPreset,
  Preview,
  Tailwind,
  Text,
} from 'react-email'

export interface Origin {
  ip: string
  device: string
  time: string
}

export function Layout({
  preview,
  title,
  team,
  year,
  children,
}: {
  preview: string
  title: string
  team: string
  year: string
  children: ReactNode
}) {
  return (
    <Html lang="en">
      <Tailwind
        config={{
          presets: [pixelBasedPreset],
          theme: {
            extend: {
              fontFamily: {
                sans: [
                  'Inter',
                  '-apple-system',
                  'BlinkMacSystemFont',
                  'Segoe UI',
                  'Helvetica',
                  'Arial',
                  'sans-serif',
                ],
              },
            },
          },
        }}
      >
        <Head />
        <Preview>{preview}</Preview>
        <Body className="m-0 bg-white font-sans">
          <Container className="max-w-[480px] px-6 py-12">
            <Text className="m-0 text-base/6 font-semibold text-zinc-950">Krill</Text>
            <Heading as="h1" className="mt-10 mb-0 text-2xl/8 font-semibold text-zinc-950">
              {title}
            </Heading>
            {children}
            <Hr className="mt-12 mb-6 border-zinc-200" />
            <Text className="m-0 text-xs/5 text-zinc-500">
              © {year} {team}
            </Text>
          </Container>
        </Body>
      </Tailwind>
    </Html>
  )
}

export function Paragraph({ children }: { children: ReactNode }) {
  return <Text className="mt-3 mb-0 text-sm/6 text-zinc-600">{children}</Text>
}

export function Strong({ children }: { children: ReactNode }) {
  return <strong className="font-semibold text-zinc-950">{children}</strong>
}

export function Code({ code }: { code: string }) {
  return (
    <Text className="mt-6 mb-0 text-4xl/10 font-semibold tracking-wide text-zinc-950">{code}</Text>
  )
}

export function NotYou({ origin, children }: { origin: Origin; children: ReactNode }) {
  return (
    <>
      <Text className="mt-12 mb-0 text-sm/6 font-semibold text-zinc-950">Didn't request this?</Text>
      <Text className="mt-1 mb-0 text-sm/6 text-zinc-600">
        This code was requested from <Strong>{origin.ip}</Strong> ({origin.device}) at{' '}
        <Strong>{origin.time}</Strong>. {children}
      </Text>
    </>
  )
}
