import { Code, Layout, NotYou, type Origin, Paragraph, Strong } from './_components/Layout'

export interface ResetPasswordProps {
  username: string
  code: string
  minutes: string
  origin: Origin
  team: string
  year: string
}

export default function ResetPassword({
  username,
  code,
  minutes,
  origin,
  team,
  year,
}: ResetPasswordProps) {
  return (
    <Layout
      preview={`${code} is your Krill password reset code`}
      title="Password reset code"
      team={team}
      year={year}
    >
      <Paragraph>
        Enter this code in Krill to set a new password for <Strong>@{username}</Strong>:
      </Paragraph>
      <Code code={code} />
      <Paragraph>
        It expires in {minutes} minutes. To protect your account, don't share it.
      </Paragraph>
      <NotYou origin={origin}>
        If you didn't make this request, you can ignore this email. Your password won't change.
      </NotYou>
    </Layout>
  )
}

// Written by hand because the generated text runs inline elements together.
export function resetPasswordText({
  username,
  code,
  minutes,
  origin,
  team,
  year,
}: ResetPasswordProps) {
  return `Password reset code

Enter this code in Krill to set a new password for @${username}:

${code}

It expires in ${minutes} minutes. To protect your account, don't share it.

Didn't request this?
This code was requested from ${origin.ip} (${origin.device}) at ${origin.time}. If you didn't make this request, you can ignore this email. Your password won't change.

© ${year} ${team}
`
}

ResetPassword.PreviewProps = {
  username: 'bucky',
  code: '904162',
  minutes: '10',
  origin: { ip: '128.104.24.71', device: 'Firefox on macOS', time: '14 October 2026, 05:04 UTC' },
  team: 'Wisconsin Autonomous Underwater Vehicles',
  year: '2026',
} satisfies ResetPasswordProps
