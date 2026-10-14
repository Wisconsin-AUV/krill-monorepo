import { Code, Layout, NotYou, type Origin, Paragraph, Strong } from './_components/Layout'

export interface SignInCodeProps {
  username: string
  code: string
  minutes: string
  origin: Origin
  team: string
  year: string
}

export default function SignInCode({
  username,
  code,
  minutes,
  origin,
  team,
  year,
}: SignInCodeProps) {
  return (
    <Layout
      preview={`${code} is your Krill sign-in code`}
      title="Sign-in code"
      team={team}
      year={year}
    >
      <Paragraph>
        Enter this code in Krill to sign in as <Strong>@{username}</Strong>:
      </Paragraph>
      <Code code={code} />
      <Paragraph>
        It expires in {minutes} minutes. To protect your account, don't share it.
      </Paragraph>
      <NotYou origin={origin}>
        If you didn't try to sign in, you can ignore this email. Nobody can sign in without the
        code.
      </NotYou>
    </Layout>
  )
}

// Written by hand because the generated text runs inline elements together.
export function signInCodeText({ username, code, minutes, origin, team, year }: SignInCodeProps) {
  return `Sign-in code

Enter this code in Krill to sign in as @${username}:

${code}

It expires in ${minutes} minutes. To protect your account, don't share it.

Didn't request this?
This code was requested from ${origin.ip} (${origin.device}) at ${origin.time}. If you didn't try to sign in, you can ignore this email. Nobody can sign in without the code.

© ${year} ${team}
`
}

SignInCode.PreviewProps = {
  username: 'bucky',
  code: '318406',
  minutes: '10',
  origin: { ip: '128.104.24.71', device: 'Firefox on macOS', time: '14 October 2026, 05:04 UTC' },
  team: 'Wisconsin Autonomous Underwater Vehicles',
  year: '2026',
} satisfies SignInCodeProps
