import { Code, Layout, NotYou, type Origin, Paragraph, Strong } from './_components/Layout'

export interface VerifyEmailProps {
  email: string
  code: string
  minutes: string
  origin: Origin
  team: string
  year: string
}

export default function VerifyEmail({
  email,
  code,
  minutes,
  origin,
  team,
  year,
}: VerifyEmailProps) {
  return (
    <Layout
      preview={`${code} is your Krill verification code`}
      title="Verification code"
      team={team}
      year={year}
    >
      <Paragraph>
        Enter this code in Krill to verify <Strong>{email}</Strong>:
      </Paragraph>
      <Code code={code} />
      <Paragraph>
        It expires in {minutes} minutes. To protect your account, don't share it.
      </Paragraph>
      <NotYou origin={origin}>If you didn't sign up for Krill, you can ignore this email.</NotYou>
    </Layout>
  )
}

// Written by hand because the generated text runs inline elements together.
export function verifyEmailText({ email, code, minutes, origin, team, year }: VerifyEmailProps) {
  return `Verification code

Enter this code in Krill to verify ${email}:

${code}

It expires in ${minutes} minutes. To protect your account, don't share it.

Didn't request this?
This code was requested from ${origin.ip} (${origin.device}) at ${origin.time}. If you didn't sign up for Krill, you can ignore this email.

© ${year} ${team}
`
}

VerifyEmail.PreviewProps = {
  email: 'bucky@wisc.edu',
  code: '457906',
  minutes: '10',
  origin: { ip: '128.104.24.71', device: 'Firefox on macOS', time: '14 October 2026, 05:04 UTC' },
  team: 'Wisconsin Autonomous Underwater Vehicles',
  year: '2026',
} satisfies VerifyEmailProps
