import { instance } from '@/api'

export interface Company {
	id: string
	name: string
	slug: string
	status: 'trial' | 'active' | 'past_due' | 'suspended' | 'closed'
	timezone: string
	locale: string
	membership_id: string
	role: 'owner' | 'admin' | 'manager' | 'agent' | 'viewer'
	membership_status: 'invited' | 'active' | 'suspended'
	created_at: string
}

export interface CompanyMember {
	membership_id: string
	user_id: string
	username: string
	email: string
	role: 'owner' | 'admin' | 'manager' | 'agent' | 'viewer'
	status: 'invited' | 'active' | 'suspended'
	joined_at: string | null
	created_at: string
}

export interface CompanyInvitation {
	id: string
	company_id: string
	email: string
	role: Exclude<CompanyMember['role'], 'owner'>
	expires_at: string
	accepted_at: string | null
	revoked_at: string | null
	created_at: string
}

export const listCompanies = () => instance.get('/v1/companies/list')

export const createCompany = (data: { name: string; slug: string; timezone?: string; locale?: string }) =>
	instance.post('/v1/companies/create', data, { fetchOptions: { successMessage: true } })

export const switchCompany = (companyId: string) =>
	instance.post('/v1/companies/switch', { company_id: companyId })

export const listMembers = () => instance.get('/v1/companies/members/list')
export const listInvitations = () => instance.get('/v1/companies/invitations/list')
export const inviteMember = (data: { email: string; role: CompanyInvitation['role'] }) =>
	instance.post('/v1/companies/invitations/create', data, { fetchOptions: { successMessage: true } })
export const changeMemberRole = (membershipId: string, role: CompanyInvitation['role']) =>
	instance.post('/v1/companies/members/change_role', { membership_id: membershipId, role }, { fetchOptions: { successMessage: true } })
export const removeMember = (membershipId: string) =>
	instance.post('/v1/companies/members/remove', { membership_id: membershipId }, { fetchOptions: { successMessage: true } })
export const revokeInvitation = (invitationId: string) =>
	instance.post('/v1/companies/invitations/revoke', { invitation_id: invitationId }, { fetchOptions: { successMessage: true } })
export const acceptInvitation = (token: string) =>
	instance.post('/v1/companies/invitations/accept', { token }, { fetchOptions: { successMessage: true } })
