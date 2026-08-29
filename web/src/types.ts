export type SiteRole = 'ADMIN' | 'STRANGER' | 'BLOCKED'
export type ServiceRole = 'ADMIN' | 'FAMILY' | 'STRANGER' | 'BLOCKED'

export const SITE_ROLES: readonly SiteRole[] = ['ADMIN', 'STRANGER', 'BLOCKED']
export const SERVICE_ROLES: readonly ServiceRole[] = ['ADMIN', 'FAMILY', 'STRANGER', 'BLOCKED']

export type User = {
  id: string | number
  google_email: string
  name: string
  roles: SiteRole
  storage_roles: ServiceRole
  resume_roles: ServiceRole
  last_seen_at: string | null
}

export function isAdmin(user: User | null | undefined): boolean {
  return user?.roles === 'ADMIN'
}

export function isBlockedRole(role: string): boolean {
  return role === 'BLOCKED'
}
