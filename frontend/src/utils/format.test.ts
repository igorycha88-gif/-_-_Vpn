import { describe, it, expect } from 'vitest'
import { formatBytes } from './format'

describe('formatBytes', () => {
  it('formats 0 bytes', () => {
    expect(formatBytes(0)).toBe('0 Б')
  })

  it('formats bytes', () => {
    expect(formatBytes(500)).toBe('500 Б')
  })

  it('formats kilobytes', () => {
    expect(formatBytes(1024)).toBe('1.0 КБ')
  })

  it('formats megabytes', () => {
    expect(formatBytes(1048576)).toBe('1.0 МБ')
  })

  it('formats gigabytes', () => {
    expect(formatBytes(1073741824)).toBe('1.0 ГБ')
  })

  it('formats terabytes', () => {
    expect(formatBytes(1099511627776)).toBe('1.0 ТБ')
  })

  it('formats fractional values', () => {
    expect(formatBytes(1536)).toBe('1.5 КБ')
  })

  it('formats large values', () => {
    expect(formatBytes(5368709120)).toBe('5.0 ГБ')
  })
})
