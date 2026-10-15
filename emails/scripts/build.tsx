import { mkdir, writeFile } from 'node:fs/promises'
import { render } from 'react-email'
import Invite, { inviteText } from '../src/invite'
import ResetPassword, { resetPasswordText } from '../src/reset-password'
import SignInCode, { signInCodeText } from '../src/sign-in-code'
import TestEmail, { testEmailText } from '../src/test-email'
import VerifyEmail, { verifyEmailText } from '../src/verify-email'

// The API fills these in with Go's html/template and text/template, so the
// props are template actions rather than values.
const shared = {
  code: '{{.Code}}',
  minutes: '{{.Minutes}}',
  origin: { ip: '{{.IP}}', device: '{{.Device}}', time: '{{.Time}}' },
  team: '{{.Team}}',
  year: '{{.Year}}',
}
const verify = { ...shared, email: '{{.Email}}' }
const account = { ...shared, username: '{{.Username}}' }
const test = { username: '{{.Username}}', team: '{{.Team}}', year: '{{.Year}}' }
const invite = {
  inviter: '{{.Inviter}}',
  role: '{{.Role}}',
  url: '{{.URL}}',
  expires: '{{.Expires}}',
  team: '{{.Team}}',
  year: '{{.Year}}',
}
const templates = {
  'verify-email': [<VerifyEmail {...verify} />, verifyEmailText(verify)],
  'reset-password': [<ResetPassword {...account} />, resetPasswordText(account)],
  'sign-in-code': [<SignInCode {...account} />, signInCodeText(account)],
  'test-email': [<TestEmail {...test} />, testEmailText(test)],
  invite: [<Invite {...invite} />, inviteText(invite)],
} as const

const out = new URL('../../api/internal/email/templates/', import.meta.url)
await mkdir(out, { recursive: true })
for (const [name, [html, text]] of Object.entries(templates)) {
  await writeFile(new URL(`${name}.html`, out), await render(html))
  await writeFile(new URL(`${name}.txt`, out), text)
}
