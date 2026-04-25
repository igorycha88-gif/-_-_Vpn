import { describe, it, expect } from 'vitest'

describe('monitoring API', () => {
  it('getTrafficAggregate is exported', async () => {
    const mod = await import('./monitoring')
    expect(typeof mod.getTrafficAggregate).toBe('function')
  })

  it('getTrafficLogs is exported', async () => {
    const mod = await import('./monitoring')
    expect(typeof mod.getTrafficLogs).toBe('function')
  })

  it('getRoutingLogs is exported', async () => {
    const mod = await import('./monitoring')
    expect(typeof mod.getRoutingLogs).toBe('function')
  })

  it('getAlerts is exported', async () => {
    const mod = await import('./monitoring')
    expect(typeof mod.getAlerts).toBe('function')
  })

  it('getMonitoringStats is exported', async () => {
    const mod = await import('./monitoring')
    expect(typeof mod.getMonitoringStats).toBe('function')
  })

  it('getPeersStats is exported', async () => {
    const mod = await import('./monitoring')
    expect(typeof mod.getPeersStats).toBe('function')
  })

  it('TrafficAggregate type has correct fields', async () => {
    const aggregate: import('./monitoring').TrafficAggregate = {
      domain: 'youtube.com',
      rx: 1024,
      tx: 512,
      count: 5,
    }
    expect(aggregate.domain).toBe('youtube.com')
    expect(aggregate.rx).toBe(1024)
    expect(aggregate.tx).toBe(512)
    expect(aggregate.count).toBe(5)
  })
})
