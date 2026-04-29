import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import ServerStatus from './ServerStatus'

describe('ServerStatus', () => {
  it('renders online status', () => {
    render(<ServerStatus title="Test Server" info={{ online: true, ip: '1.2.3.4' }} />)
    expect(screen.getByText('Test Server')).toBeInTheDocument()
    expect(screen.getByText('Онлайн')).toBeInTheDocument()
  })

  it('renders offline status', () => {
    render(<ServerStatus title="Down Server" info={{ online: false }} />)
    expect(screen.getByText('Офлайн')).toBeInTheDocument()
  })

  it('renders IP when provided', () => {
    render(<ServerStatus title="Server" info={{ online: true, ip: '10.0.0.1' }} />)
    expect(screen.getByText(/10\.0\.0\.1/)).toBeInTheDocument()
  })

  it('does not render IP when not provided', () => {
    const { container } = render(<ServerStatus title="Server" info={{ online: true }} />)
    expect(container.querySelector('p')).toBeNull()
  })

  it('renders uptime when provided', () => {
    render(<ServerStatus title="Server" info={{ online: true, uptime: '30 days' }} />)
    expect(screen.getByText(/30 days/)).toBeInTheDocument()
  })

  it('renders CPU usage when provided', () => {
    render(<ServerStatus title="Server" info={{ online: true, cpu_usage: '45%' }} />)
    expect(screen.getByText(/45%/)).toBeInTheDocument()
  })
})
