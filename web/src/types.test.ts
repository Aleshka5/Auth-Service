import { describe, expect, it } from 'vitest'
import { isAdmin, isBlockedRole } from './types'
import { admin, stranger } from './test/fixtures'

describe('isAdmin', () => {
  it('is true only for ADMIN role', () => {
    expect(isAdmin(admin)).toBe(true)
    expect(isAdmin(stranger)).toBe(false)
    expect(isAdmin({ ...admin, roles: 'BLOCKED' })).toBe(false)
    expect(isAdmin(null)).toBe(false)
  })
})

describe('isBlockedRole', () => {
  it('matches BLOCKED', () => {
    expect(isBlockedRole('BLOCKED')).toBe(true)
    expect(isBlockedRole('STRANGER')).toBe(false)
  })
})
