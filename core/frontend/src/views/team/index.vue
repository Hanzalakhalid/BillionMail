<template>
	<div class="p-24px">
		<div class="flex items-center justify-between mb-20px">
			<div>
				<h2 class="m-0">Team members</h2>
				<p class="text-gray-500 mt-6px">Manage access to the active company workspace.</p>
			</div>
			<n-button type="primary" @click="inviteOpen = true">Invite member</n-button>
		</div>

		<n-tabs type="line">
			<n-tab-pane name="members" tab="Members">
				<n-data-table :columns="memberColumns" :data="members" :loading="loading" :row-key="row => row.membership_id" />
			</n-tab-pane>
			<n-tab-pane name="invitations" tab="Pending invitations">
				<n-data-table :columns="invitationColumns" :data="pendingInvitations" :loading="loading" :row-key="row => row.id" />
			</n-tab-pane>
		</n-tabs>

		<n-modal v-model:show="inviteOpen" preset="card" title="Invite team member" class="max-w-520px">
			<n-form label-placement="top">
				<n-form-item label="Email"><n-input v-model:value="inviteForm.email" placeholder="person@company.com" /></n-form-item>
				<n-form-item label="Role"><n-select v-model:value="inviteForm.role" :options="roleOptions" /></n-form-item>
			</n-form>
			<template #footer><div class="flex justify-end gap-8px"><n-button @click="inviteOpen = false">Cancel</n-button><n-button type="primary" :loading="saving" @click="submitInvite">Send invitation</n-button></div></template>
		</n-modal>

		<n-modal v-model:show="roleOpen" preset="card" title="Change member role" class="max-w-520px">
			<n-select v-model:value="roleForm.role" :options="roleOptions" />
			<template #footer><div class="flex justify-end gap-8px"><n-button @click="roleOpen = false">Cancel</n-button><n-button type="primary" :loading="saving" @click="submitRole">Save role</n-button></div></template>
		</n-modal>
	</div>
</template>

<script lang="tsx" setup>
import { NButton, NTag, type DataTableColumns } from 'naive-ui'
import { confirm } from '@/utils'
import {
	changeMemberRole, inviteMember, listInvitations, listMembers, removeMember, revokeInvitation,
	type CompanyInvitation, type CompanyMember,
} from '@/api/modules/companies'

const members = ref<CompanyMember[]>([])
const invitations = ref<CompanyInvitation[]>([])
const loading = ref(false)
const saving = ref(false)
const inviteOpen = ref(false)
const roleOpen = ref(false)
const inviteForm = reactive({ email: '', role: 'agent' as CompanyInvitation['role'] })
const roleForm = reactive({ membershipId: '', role: 'agent' as CompanyInvitation['role'] })
const roleOptions = ['admin', 'manager', 'agent', 'viewer'].map(value => ({ label: value[0].toUpperCase() + value.slice(1), value }))
const pendingInvitations = computed(() => invitations.value.filter(item => !item.accepted_at && !item.revoked_at))

const load = async () => {
	loading.value = true
	try {
		const [memberResult, invitationResult] = await Promise.all([listMembers(), listInvitations()])
		members.value = (memberResult as { members?: CompanyMember[] })?.members ?? []
		invitations.value = (invitationResult as { invitations?: CompanyInvitation[] })?.invitations ?? []
	} finally { loading.value = false }
}

const submitInvite = async () => {
	if (!inviteForm.email.trim()) return
	saving.value = true
	try { await inviteMember({ email: inviteForm.email.trim(), role: inviteForm.role }); inviteOpen.value = false; inviteForm.email = ''; await load() }
	finally { saving.value = false }
}

const openRole = (member: CompanyMember) => { roleForm.membershipId = member.membership_id; roleForm.role = member.role === 'owner' ? 'admin' : member.role; roleOpen.value = true }
const submitRole = async () => { saving.value = true; try { await changeMemberRole(roleForm.membershipId, roleForm.role); roleOpen.value = false; await load() } finally { saving.value = false } }
const confirmRemove = (member: CompanyMember) => confirm({ title: 'Remove member', content: `Remove ${member.username || member.email} from this workspace?`, onConfirm: async () => { await removeMember(member.membership_id); await load() } })
const confirmRevoke = (invitation: CompanyInvitation) => confirm({ title: 'Revoke invitation', content: `Revoke the invitation for ${invitation.email}?`, onConfirm: async () => { await revokeInvitation(invitation.id); await load() } })

const memberColumns: DataTableColumns<CompanyMember> = [
	{ title: 'User', key: 'username', render: row => <div><div>{row.username}</div><small class="text-gray-500">{row.email}</small></div> },
	{ title: 'Role', key: 'role', render: row => <NTag size="small">{row.role}</NTag> },
	{ title: 'Status', key: 'status' },
	{ title: 'Actions', key: 'actions', render: row => row.role === 'owner' ? null : <div class="flex gap-6px"><NButton size="small" onClick={() => openRole(row)}>Change role</NButton><NButton size="small" type="error" ghost onClick={() => confirmRemove(row)}>Remove</NButton></div> },
]
const invitationColumns: DataTableColumns<CompanyInvitation> = [
	{ title: 'Email', key: 'email' }, { title: 'Role', key: 'role' }, { title: 'Expires', key: 'expires_at', render: row => new Date(row.expires_at).toLocaleString() },
	{ title: 'Actions', key: 'actions', render: row => <NButton size="small" type="error" ghost onClick={() => confirmRevoke(row)}>Revoke</NButton> },
]

onMounted(load)
</script>
