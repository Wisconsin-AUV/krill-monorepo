import { Layout, Paragraph, Strong } from './_components/Layout'

export interface TestEmailProps {
  username: string
  team: string
  year: string
}

export default function TestEmail({ username, team, year }: TestEmailProps) {
  return (
    <Layout preview="Krill can send email" title="Email works" team={team} year={year}>
      <Paragraph>
        <Strong>@{username}</Strong> sent this from the admin panel. Verification, password reset,
        and sign-in codes will arrive the same way.
      </Paragraph>
    </Layout>
  )
}

export function testEmailText({ username, team, year }: TestEmailProps) {
  return `Email works

@${username} sent this from the admin panel. Verification, password reset, and sign-in codes will arrive the same way.

© ${year} ${team}
`
}

TestEmail.PreviewProps = {
  username: 'bucky',
  team: 'Wisconsin Autonomous Underwater Vehicles',
  year: '2026',
} satisfies TestEmailProps
