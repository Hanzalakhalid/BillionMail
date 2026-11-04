<template>
	<AuthShell title="Choose a new password" description="Reset links expire after one hour and can be used once.">
		<n-alert v-if="!token" type="error" class="mb-4">This reset link is invalid.</n-alert>
		<n-form ref="formRef" :model="form" :rules="rules" size="large">
			<n-form-item path="password" label="New password"><n-input v-model:value="form.password" type="password" show-password-on="click" autocomplete="new-password" /></n-form-item>
			<n-form-item path="confirmation" label="Confirm password"><n-input v-model:value="form.confirmation" type="password" show-password-on="click" autocomplete="new-password" /></n-form-item>
			<n-button type="primary" block :disabled="!token" :loading="loading" @click="submit">Reset password</n-button>
		</n-form>
		<div class="auth-links"><router-link to="/login">Back to sign in</router-link></div>
	</AuthShell>
</template>

<script setup lang="ts">
import AuthShell from './components/AuthShell.vue'
import { resetPassword } from '@/api/modules/identity'

const route = useRoute()
const router = useRouter()
const token = computed(() => typeof route.query.token === 'string' ? route.query.token : '')
const formRef = useTemplateRef('formRef')
const loading = ref(false)
const form = reactive({ password: '', confirmation: '' })
const rules = {
	password: { required: true, pattern: /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{12,128}$/, message: 'Use 12+ characters with uppercase, lowercase, and a number', trigger: 'blur' },
	confirmation: { required: true, validator: () => form.confirmation === form.password, message: 'Passwords must match', trigger: 'blur' },
}
const submit = async () => {
	await formRef.value?.validate()
	loading.value = true
	try { await resetPassword(token.value, form.password); await router.push('/login') } finally { loading.value = false }
}
</script>
