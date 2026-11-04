<template>
	<AuthShell title="Reset your password" description="We will send reset instructions if the account exists.">
		<n-form ref="formRef" :model="form" :rules="rules" size="large">
			<n-form-item path="email" label="Email"><n-input v-model:value="form.email" autocomplete="email" /></n-form-item>
			<n-button type="primary" block :loading="loading" @click="submit">Send reset instructions</n-button>
		</n-form>
		<div class="auth-links"><router-link to="/login">Back to sign in</router-link></div>
	</AuthShell>
</template>

<script setup lang="ts">
import AuthShell from './components/AuthShell.vue'
import { requestPasswordReset } from '@/api/modules/identity'

const formRef = useTemplateRef('formRef')
const loading = ref(false)
const form = reactive({ email: '' })
const rules = { email: { required: true, type: 'email', message: 'Enter a valid email', trigger: 'blur' } }
const submit = async () => {
	await formRef.value?.validate()
	loading.value = true
	try { await requestPasswordReset(form.email) } finally { loading.value = false }
}
</script>
