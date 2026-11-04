<template>
	<AuthShell title="Workspace invitation" :description="message">
		<n-spin v-if="loading" class="block mx-auto" />
		<div v-else class="auth-links"><router-link to="/">Go to workspace</router-link></div>
	</AuthShell>
</template>

<script setup lang="ts">
import AuthShell from './components/AuthShell.vue'
import { acceptInvitation } from '@/api/modules/companies'

const route = useRoute()
const loading = ref(true)
const message = ref('Accepting your secure invitation…')
onMounted(async () => {
	const token = typeof route.query.token === 'string' ? route.query.token : ''
	if (!token) { message.value = 'This invitation link is invalid.'; loading.value = false; return }
	try { await acceptInvitation(token); message.value = 'Invitation accepted. The workspace is now available in your switcher.' }
	catch { message.value = 'This invitation is invalid, expired, or belongs to another email address.' }
	finally { loading.value = false }
})
</script>
