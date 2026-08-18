import { describe, expect, it } from 'vitest'
import { isAdmin } from './types'
import { admin, stranger } from './test/fixtures'

describe('isAdmin', () => {
  it('is true only for ADMIN role', () => {
    expect(isAdmin(admin)).toBe(true)
    expect(isAdmin(stranger)).toBe(false)
    expect(isAdmin(null)).toBe(false)
  })
})
