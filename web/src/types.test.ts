import { describe, expect, it } from 'vitest'
import { isAdmin, isBlockedRole, serviceRole, withGlobalRole, withServiceRole } from './types'
import { admin, stranger } from './test/fixtures'

describe('isAdmin', () => {
  it('is true only for global ADMIN', () => {
    expect(isAdmin(admin)).toBe(true)
    expect(isAdmin(stranger)).toBe(false)
    expect(isAdmin(withGlobalRole(admin, 'BLOCKED'))).toBe(false)
    expect(isAdmin(null)).toBe(false)
  })
})

describe('isBlockedRole', () => {
  it('matches BLOCKED', () => {
    expect(isBlockedRole('BLOCKED')).toBe(true)
    expect(isBlockedRole('STRANGER')).toBe(false)
  })
})

describe('serviceRole', () => {
  it('reads a persisted service row', () => {
    expect(serviceRole(stranger, 'storage')).toBe('FAMILY')
  })

  it('treats a missing service as STRANGER', () => {
    expect(serviceRole(stranger, 'resume')).toBe('STRANGER')
    expect(serviceRole({ services: [] }, 'storage')).toBe('STRANGER')
  })

  it('withServiceRole replaces that service only', () => {
    const next = withServiceRole(stranger, 'resume', 'ADMIN')
    expect(serviceRole(next, 'resume')).toBe('ADMIN')
    expect(serviceRole(next, 'storage')).toBe('FAMILY')
  })
})
