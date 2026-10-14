import { mkdir, writeFile } from 'node:fs/promises'
import { render } from 'react-email'
import ResetPassword, { resetPasswordText } from '../src/reset-password'
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
const reset = { ...shared, username: '{{.Username}}' }
const templates = {
  'verify-email': [<VerifyEmail {...verify} />, verifyEmailText(verify)],
  'reset-password': [<ResetPassword {...reset} />, resetPasswordText(reset)],
} as const

const out = new URL('../../api/internal/email/templates/', import.meta.url)
await mkdir(out, { recursive: true })
for (const [name, [html, text]] of Object.entries(templates)) {
  await writeFile(new URL(`${name}.html`, out), await render(html))
  await writeFile(new URL(`${name}.txt`, out), text)
}
