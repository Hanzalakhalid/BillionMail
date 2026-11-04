<template>
	<AuthShell title="Verify your email" :description="message">
		<n-spin v-if="loading" class="block mx-auto" />
		<div v-else class="auth-links"><router-link to="/login">Continue to sign in</router-link></div>
	</AuthShell>
</template>

<script setup lang="ts">
import AuthShell from './components/AuthShell.vue'
import { verifyEmail } from '@/api/modules/identity'

const route = useRoute()
const loading = ref(true)
const message = ref('Verifying your secure link…')
onMounted(async () => {
	const token = typeof route.query.token === 'string' ? route.query.token : ''
	if (!token) { message.value = 'This verification link is invalid.'; loading.value = false; return }
	try { await verifyEmail(token); message.value = 'Your email is verified. You can now sign in.' }
	catch { message.value = 'This verification link is invalid or expired.' }
	finally { loading.value = false }
})
</script>
