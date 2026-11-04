<template>
	<AuthShell title="Create your workspace" description="Start your secure 14-day trial.">
		<n-form ref="formRef" :model="form" :rules="rules" size="large">
			<n-form-item path="company_name" label="Company name"><n-input v-model:value="form.company_name" /></n-form-item>
			<n-form-item path="username" label="Username"><n-input v-model:value="form.username" autocomplete="username" /></n-form-item>
			<n-form-item path="email" label="Work email"><n-input v-model:value="form.email" autocomplete="email" /></n-form-item>
			<n-form-item path="password" label="Password">
				<n-input v-model:value="form.password" type="password" show-password-on="click" autocomplete="new-password" />
			</n-form-item>
			<n-button type="primary" block :loading="loading" @click="submit">Create workspace</n-button>
		</n-form>
		<div class="auth-links"><router-link to="/login">Back to sign in</router-link></div>
	</AuthShell>
</template>

<script setup lang="ts">
import AuthShell from './components/AuthShell.vue'
import { register } from '@/api/modules/identity'

const router = useRouter()
const formRef = useTemplateRef('formRef')
const loading = ref(false)
const form = reactive({ company_name: '', username: '', email: '', password: '', timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC', locale: navigator.language.split('-')[0] || 'en' })
const rules = {
	company_name: { required: true, message: 'Company name is required', trigger: 'blur' },
	username: { required: true, pattern: /^[A-Za-z0-9][A-Za-z0-9._-]{2,63}$/, message: 'Use 3–64 letters, numbers, dots, underscores, or hyphens', trigger: 'blur' },
	email: { required: true, type: 'email', message: 'Enter a valid email', trigger: 'blur' },
	password: { required: true, pattern: /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{12,128}$/, message: 'Use 12+ characters with uppercase, lowercase, and a number', trigger: 'blur' },
}
const submit = async () => {
	await formRef.value?.validate()
	loading.value = true
	try { await register(toRaw(form)); await router.push('/login') } finally { loading.value = false }
}
</script>
