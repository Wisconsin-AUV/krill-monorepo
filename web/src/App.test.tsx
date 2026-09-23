import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import App from './App'

describe('App', () => {
  it('renders the navigation', async () => {
    render(<App />)
    expect(await screen.findAllByText('Videos')).not.toHaveLength(0)
  })
})
