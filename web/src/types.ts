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

/** A service runtime may sit at one of three operator-selectable states. */
export type ServiceState = 'off' | 'cpu' | 'gpu'

/** Where a service actually is right now, including the in-between states. */
export type ServiceRuntimeState = 'off' | 'starting' | 'ready' | 'stopping' | 'failed'

export type HostMemory = {
  total_mib: number
  used_mib: number
  reserve_mib: number
  budget_mib: number
  claimed_mib: number
  free_mib: number
  available_mib?: number
}

export type HostCpu = {
  threads: number
  load1: number
}

export type HostDevice = {
  index: number
  name: string
  compute_cap?: string
}

/** Host memory budget for model services; `vram` is null on a machine without a GPU. */
export type HostBudget = {
  ram: HostMemory
  vram: HostMemory | null
  cpu?: HostCpu
  device?: HostDevice | null
  sampled_at?: string
  sample_source?: string
}

export type ServiceClaim = {
  ram_mib: number
  vram_mib: number
}

export type ServiceProfile = {
  available: boolean
  reason: string | null
  claim: ServiceClaim
}

export type ServiceTransition = {
  phase: string
  target_state?: ServiceState
  started_at?: string
}

/** One model service as reported by GET /api/services. */
export type ManagedService = {
  id: string
  title: string
  kind?: string
  state: ServiceRuntimeState
  mode: ServiceState | null
  desired_state: ServiceState
  pending_state?: ServiceState | null
  busy: boolean
  supported_states?: ServiceState[]
  vertices?: ServiceState[]
  transitions_allowed?: ServiceState[]
  retry_state?: ServiceState | null
  unavailable_states?: Partial<Record<ServiceState, { reason: string } | undefined>>
  preemptible?: boolean
  options?: Record<string, unknown>
  claim: ServiceClaim
  profiles?: Partial<Record<ServiceState, ServiceProfile>>
  measured?: { ram_mib: number | null; vram_mib: number | null; at?: string } | null
  over_budget?: boolean
  container?: Record<string, unknown> | null
  health?: { ok: boolean; checked_at?: string; detail?: string } | null
  consumers?: string[]
  notes?: string[]
  drift?: boolean
  fail_sticky?: boolean
  last_error?: string | null
  transition?: ServiceTransition | null
}

export type ServicesSnapshot = {
  host: HostBudget
  services: ManagedService[]
  active_transitions?: unknown[]
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
