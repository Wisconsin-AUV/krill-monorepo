import { Button, Link } from 'react-email'
import { Layout, Paragraph, Strong } from './_components/Layout'

export interface InviteProps {
  inviter: string
  role: string
  url: string
  expires: string
  team: string
  year: string
}

export default function Invite({ inviter, role, url, expires, team, year }: InviteProps) {
  return (
    <Layout
      preview={`${inviter} invited you to Krill`}
      title="You're invited to Krill"
      team={team}
      year={year}
    >
      <Paragraph>
        <Strong>{inviter}</Strong> invited you to join {team} on Krill as a {role}.
      </Paragraph>
      <Button
        href={url}
        className="mt-6 rounded-lg bg-sky-600 px-4 py-2.5 text-sm/6 font-semibold text-white"
      >
        Create your account
      </Button>
      <Paragraph>The link expires {expires}.</Paragraph>
      <Paragraph>
        Or open{' '}
        <Link href={url} className="break-all text-sky-600">
          {url}
        </Link>
      </Paragraph>
    </Layout>
  )
}

export function inviteText({ inviter, role, url, expires, team, year }: InviteProps) {
  return `You're invited to Krill

${inviter} invited you to join ${team} on Krill as a ${role}.

Create your account:
${url}

The link expires ${expires}.

© ${year} ${team}
`
}

Invite.PreviewProps = {
  inviter: 'Bucky Badger',
  role: 'labeler',
  url: 'https://krill.example.com/register?invite=3q2-7wEAAAA',
  expires: 'on 21 October 2026',
  team: 'Wisconsin Autonomous Underwater Vehicles',
  year: '2026',
} satisfies InviteProps
