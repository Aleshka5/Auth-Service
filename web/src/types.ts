export type SiteRole = 'ADMIN' | 'STRANGER'
export type ServiceRole = 'ADMIN' | 'FAMILY' | 'STRANGER'

export const SITE_ROLES: readonly SiteRole[] = ['ADMIN', 'STRANGER']
export const SERVICE_ROLES: readonly ServiceRole[] = ['ADMIN', 'FAMILY', 'STRANGER']

export type User = {
  id: string | number
  google_email: string
  name: string
  roles: SiteRole
  storage_roles: ServiceRole
  resume_roles: ServiceRole
  blocked: boolean
  last_seen_at: string | null
}

export function isAdmin(user: User | null | undefined): boolean {
  return user?.roles === 'ADMIN'
}
