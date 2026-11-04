import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api', () => ({ instance: { get, post } }))

import { createCompany, inviteMember, listCompanies, listMembers, removeMember, switchCompany } from './companies'

describe('companies API', () => {
	beforeEach(() => vi.clearAllMocks())
	it('lists memberships', () => {
		listCompanies()
		expect(get).toHaveBeenCalledWith('/v1/companies/list')
	})
	it('creates a workspace', () => {
		const data = { name: 'Acme', slug: 'acme' }
		createCompany(data)
		expect(post).toHaveBeenCalledWith('/v1/companies/create', data, expect.any(Object))
	})
	it('switches the active workspace', () => {
		switchCompany('company-id')
		expect(post).toHaveBeenCalledWith('/v1/companies/switch', { company_id: 'company-id' })
	})
	it('lists company members', () => {
		listMembers()
		expect(get).toHaveBeenCalledWith('/v1/companies/members/list')
	})
	it('invites a member without exposing an invitation token', () => {
		inviteMember({ email: 'agent@example.com', role: 'agent' })
		expect(post).toHaveBeenCalledWith('/v1/companies/invitations/create', { email: 'agent@example.com', role: 'agent' }, expect.any(Object))
	})
	it('removes a membership by scoped identifier', () => {
		removeMember('membership-id')
		expect(post).toHaveBeenCalledWith('/v1/companies/members/remove', { membership_id: 'membership-id' }, expect.any(Object))
	})
})
