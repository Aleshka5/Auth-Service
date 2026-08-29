export type Role = 'ADMIN' | 'FAMILY' | 'STRANGER' | 'BLOCKED'
export type SiteRole = Role
export type ServiceRole = Role

export const SITE_ROLES: readonly SiteRole[] = ['ADMIN', 'FAMILY', 'STRANGER', 'BLOCKED']
export const SERVICE_ROLES: readonly ServiceRole[] = ['ADMIN', 'FAMILY', 'STRANGER', 'BLOCKED']

export type ServiceRoleEntry = {
  service: string
  role: ServiceRole
}

/** SPA contract for GET /api/me (§8.1). Missing services imply STRANGER. */
export type User = {
  id: string
  google_sub: string
  email: string
  username: string
  global_role: Role
  services: ServiceRoleEntry[]
}

/** User-Service GET /users element. Timestamps are ignored by the SPA. */
export type UserListItem = User & {
  created_at?: string
  updated_at?: string
}

export type UserList = {
  users: UserListItem[]
}

export function isAdmin(user: User | null | undefined): boolean {
  return user?.global_role === 'ADMIN'
}

export function isBlockedRole(role: string): boolean {
  return role === 'BLOCKED'
}

export function serviceRole(user: Pick<User, 'services'>, service: string): ServiceRole {
  return user.services?.find((row) => row.service === service)?.role ?? 'STRANGER'
}

export function withGlobalRole(user: User, role: Role): User {
  return { ...user, global_role: role }
}

export function withServiceRole(user: User, service: string, role: ServiceRole): User {
  const rest = (user.services ?? []).filter((row) => row.service !== service)
  return { ...user, services: [...rest, { service, role }] }
}
