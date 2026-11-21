import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Link, MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { RouteTransition } from './route-transition'

function App() {
  return (
    <MemoryRouter initialEntries={['/list']}>
      <Routes>
        <Route element={<RouteTransition />}>
          <Route path="/list" element={<Link to="/detail">go</Link>} />
          <Route path="/detail" element={<Link to="/detail?tab=x">filter</Link>} />
        </Route>
      </Routes>
    </MemoryRouter>
  )
}

describe('RouteTransition', () => {
  it('fades with opacity only and respects reduced motion', () => {
    render(<App />)
    const cls = screen.getByTestId('route-transition').className
    expect(cls).toContain('fade-in-0')
    expect(cls).toContain('motion-reduce:animate-none')
    expect(cls).not.toMatch(/slide|zoom|spin/)
  })

  it('remounts on pathname change but not on query change', async () => {
    const user = userEvent.setup()
    render(<App />)
    const first = screen.getByTestId('route-transition')
    await user.click(screen.getByText('go'))
    const second = screen.getByTestId('route-transition')
    expect(second).not.toBe(first)
    await user.click(screen.getByText('filter'))
    expect(screen.getByTestId('route-transition')).toBe(second)
  })
})
