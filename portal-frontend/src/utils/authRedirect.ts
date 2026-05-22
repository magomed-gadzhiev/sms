import type { UserRole } from '../contexts/AuthContext';

// Returns the post-auth landing route for a given role. Single source of truth
// for login/register/2FA redirects — keeps admin (and any future privileged
// roles) out of the client portal at `/dashboard` → `/command-center`.
//
// If a new value is added to UserRole, TypeScript fails at this call site and
// forces a deliberate routing decision rather than silently dumping the user
// at /dashboard.
export function destForRole(role: UserRole | undefined): string {
  return role === 'admin' || role === 'superadmin' ? '/admin' : '/dashboard';
}
